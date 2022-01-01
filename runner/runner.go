// Package runner implements local avalanchego runner.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-network-runner/api"
	"github.com/ava-labs/avalanche-network-runner/local"
	"github.com/ava-labs/avalanche-network-runner/network"
	"github.com/ava-labs/avalanche-network-runner/network/node"
	avago_api "github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/fatih/color"
)

func Run(
	avalancheGoBinPath string,
	vmName string,
	vmID string,
	vmGenesisPath string,
	outputPath string) (err error) {
	lc := newLocalNetwork(avalancheGoBinPath, vmName, vmID, vmGenesisPath, outputPath)

	go lc.start()
	select {
	case <-lc.readyc:
		color.Green("cluster is ready, waiting for signal/error")
	case s := <-lc.sigc:
		color.Red("received signal %v before ready, shutting down", s)
		lc.shutdown()
		return nil
	}
	select {
	case s := <-lc.sigc:
		color.Red("received signal %v, shutting down", s)
	case err = <-lc.errc:
		color.Red("received error %v, shutting down", err)
	}

	lc.shutdown()
	return err
}

type localNetwork struct {
	logger  logging.Logger
	logsDir string

	cfg network.Config

	binPath       string
	vmName        string
	vmID          string
	vmGenesisPath string
	outputPath    string

	nw network.Network

	nodes     map[string]node.Node
	nodeNames []string
	nodeIDs   map[string]string
	uris      map[string]string
	apiClis   map[string]api.Client

	xchainPreFundedAddr string
	pchainPreFundedAddr string
	cchainPreFundedAddr string

	xchainFundedAddrs map[string]string
	pchainFundedAddrs map[string]string
	cchainFundedAddrs map[string]string

	subnetTxID   ids.ID // tx ID for "create subnet"
	blkChainTxID ids.ID // tx ID for "create blockchain"

	readyc          chan struct{} // closed when local network is ready/healthy
	readycCloseOnce sync.Once

	sigc  chan os.Signal
	stopc chan struct{}
	donec chan struct{}
	errc  chan error
}

func newLocalNetwork(
	avalancheGoBinPath string,
	vmName string,
	vmID string,
	vmGenesisPath string,
	outputPath string,
) *localNetwork {
	lcfg, err := logging.DefaultConfig()
	if err != nil {
		panic(err)
	}
	logFactory := logging.NewFactory(lcfg)
	logger, err := logFactory.Make("main")
	if err != nil {
		panic(err)
	}

	logsDir, err := ioutil.TempDir(os.TempDir(), "runnerlogs")
	if err != nil {
		panic(err)
	}

	cfg := local.NewDefaultConfig(avalancheGoBinPath)
	nodeNames := make([]string, len(cfg.NodeConfigs))
	for i := range cfg.NodeConfigs {
		nodeName := fmt.Sprintf("node%d", i+1)

		nodeNames[i] = nodeName
		cfg.NodeConfigs[i].Name = nodeName

		// need to whitelist subnet ID to create custom VM chain
		// ref. vms/platformvm/createChain
		cfg.NodeConfigs[i].ConfigFile = []byte(fmt.Sprintf(`{
	"network-peer-list-gossip-frequency":"250ms",
	"network-max-reconnect-delay":"1s",
	"public-ip":"127.0.0.1",
	"health-check-frequency":"2s",
	"api-admin-enabled":true,
	"api-ipcs-enabled":true,
	"index-enabled":true,
	"log-display-level":"INFO",
	"log-level":"INFO",
	"log-dir":"%s",
	"whitelisted-subnets":"%s"
}`,
			filepath.Join(logsDir, nodeName),
			expectedSubnetTxID,
		))
		wr := &writer{
			col:  colors[i%len(cfg.NodeConfigs)],
			name: nodeName,
			w:    os.Stdout,
		}
		cfg.NodeConfigs[i].ImplSpecificConfig = local.NodeConfig{
			BinaryPath: avalancheGoBinPath,
			Stdout:     wr,
			Stderr:     wr,
		}
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	return &localNetwork{
		logger:  logger,
		logsDir: logsDir,

		cfg: cfg,

		binPath:       avalancheGoBinPath,
		vmName:        vmName,
		vmID:          vmID,
		vmGenesisPath: vmGenesisPath,
		outputPath:    outputPath,

		nodeNames: nodeNames,
		nodeIDs:   make(map[string]string),
		uris:      make(map[string]string),
		apiClis:   make(map[string]api.Client),

		xchainFundedAddrs: make(map[string]string),
		pchainFundedAddrs: make(map[string]string),
		cchainFundedAddrs: make(map[string]string),

		readyc: make(chan struct{}),
		sigc:   sigc,
		stopc:  make(chan struct{}),
		donec:  make(chan struct{}),
		errc:   make(chan error, 1),
	}
}

func (lc *localNetwork) start() {
	defer func() {
		close(lc.donec)
	}()

	color.Blue("create and run local network with log-dir %q", lc.logsDir)
	nw, err := local.NewNetwork(lc.logger, lc.cfg)
	if err != nil {
		lc.errc <- err
		return
	}
	lc.nw = nw

	if err := lc.waitForHealthy(); err != nil {
		lc.errc <- err
		return
	}

	if err := lc.createUser(); err != nil {
		lc.errc <- err
		return
	}
	if err := lc.importKeysAndFunds(); err != nil {
		lc.errc <- err
		return
	}
	for _, name := range lc.nodeNames {
		if err := lc.checkXChainAddress(name, lc.xchainPreFundedAddr); err != nil {
			lc.errc <- err
			return
		}
		if err := lc.checkPChainAddress(name, lc.pchainPreFundedAddr); err != nil {
			lc.errc <- err
			return
		}
	}
	if err := lc.createAddresses(); err != nil {
		lc.errc <- err
		return
	}
	for name, addr := range lc.xchainFundedAddrs {
		if err := lc.checkXChainAddress(name, addr); err != nil {
			lc.errc <- err
			return
		}
	}
	for name, addr := range lc.pchainFundedAddrs {
		if err := lc.checkPChainAddress(name, addr); err != nil {
			lc.errc <- err
			return
		}
	}
	if err := lc.transferFunds(); err != nil {
		lc.errc <- err
		return
	}

	enableSubnet := lc.vmID != "" && lc.vmGenesisPath != ""
	if enableSubnet {
		if err := lc.createSubnet(); err != nil {
			lc.errc <- err
			return
		}
		for _, name := range lc.nodeNames {
			if err := lc.checkPChainTx(name, lc.subnetTxID); err != nil {
				lc.errc <- err
				return
			}
			if err := lc.checkSubnet(name); err != nil {
				lc.errc <- err
				return
			}
		}
		if err := lc.addSubnetValidators(); err != nil {
			lc.errc <- err
			return
		}
		if err := lc.createBlockchain(); err != nil {
			lc.errc <- err
			return
		}
		for _, name := range lc.nodeNames {
			if err := lc.checkPChainTx(name, lc.blkChainTxID); err != nil {
				lc.errc <- err
				return
			}
			if err := lc.checkBlockchain(name); err != nil {
				lc.errc <- err
				return
			}
		}
		for _, name := range lc.nodeNames {
			if err := lc.checkBlockchainBootstrapped(name); err != nil {
				lc.errc <- err
				return
			}
		}
	}

	if err := lc.writeOutput(); err != nil {
		lc.errc <- err
		return
	}
}

const (
	genesisPrivKey = "PrivateKey-ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"

	healthyWait   = 2 * time.Minute
	txConfirmWait = time.Minute

	checkInterval = time.Second

	validatorWeight    = 50
	validatorStartDiff = 30 * time.Second
	validatorEndDiff   = 30 * 24 * time.Hour // 30 days
)

var errAborted = errors.New("aborted")

func (lc *localNetwork) waitForHealthy() error {
	color.Blue("waiting for all nodes to report healthy...")

	ctx, cancel := context.WithTimeout(context.Background(), healthyWait)
	defer cancel()
	hc := lc.nw.Healthy(ctx)
	select {
	case <-lc.stopc:
		return errAborted
	case <-ctx.Done():
		return ctx.Err()
	case err := <-hc:
		if err != nil {
			return err
		}
	}

	nodes, err := lc.nw.GetAllNodes()
	if err != nil {
		return err
	}
	lc.nodes = nodes

	for nodeName, node := range nodes {
		nodeID := node.GetNodeID().PrefixedString(avago_constants.NodeIDPrefix)
		lc.nodeIDs[nodeName] = nodeID

		uri := fmt.Sprintf("http://%s:%d", node.GetURL(), node.GetAPIPort())
		lc.uris[nodeName] = uri

		lc.apiClis[nodeName] = node.GetAPIClient()
		color.Cyan("%s: node ID %q, URI %q", nodeName, nodeID, uri)
	}

	lc.readycCloseOnce.Do(func() {
		close(lc.readyc)
	})
	return nil
}

var (
	// need to hard-code user-pass in order to
	// determine subnet ID for whitelisting
	userPass = avago_api.UserPass{
		Username: "test",
		Password: "vmsrkewl",
	}

	// expected response from "ImportKey"
	// based on hard-coded "userPass" and "genesisPrivKey"
	expectedXchainFundedAddr = "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedPchainFundedAddr = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedCchainFundedAddr = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

	// expected response from "CreateSubnet"
	// based on hard-coded "userPass" and "pchainFundedAddr"
	expectedSubnetTxID = "24tZhrm8j8GCJRE9PomW8FaeqbgGS4UAQjJnqqn8pq5NwYSYV1"
)

func (lc *localNetwork) createUser() error {
	color.Blue("setting up the same user in all nodes...")
	for nodeName, cli := range lc.apiClis {
		ok, err := cli.KeystoreAPI().CreateUser(userPass)
		if !ok || err != nil {
			return fmt.Errorf("failedt to create user: %w in %q", err, nodeName)
		}
	}
	return nil
}

func (lc *localNetwork) importKeysAndFunds() error {
	color.Blue("importing genesis key and funds to the user in all nodes...")
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		xAddr, err := cli.XChainAPI().ImportKey(userPass, genesisPrivKey)
		if err != nil {
			return fmt.Errorf("failed to import genesis key for X-chain: %w in %q", err, nodeName)
		}
		lc.xchainPreFundedAddr = xAddr
		xBalance, err := cli.XChainAPI().GetBalance(xAddr, "AVAX", false)
		if err != nil {
			return fmt.Errorf("failed to get X-chain balance: %w in %q", err, nodeName)
		}
		if lc.xchainPreFundedAddr != expectedXchainFundedAddr {
			return fmt.Errorf("unexpected X-chain funded address %q (expected %q)", lc.xchainPreFundedAddr, expectedXchainFundedAddr)
		}
		color.Cyan("funded X-chain: address %q, balance %d $AVAX in %q", xAddr, xBalance.Balance, nodeName)

		pAddr, err := cli.PChainAPI().ImportKey(userPass, genesisPrivKey)
		if err != nil {
			return fmt.Errorf("failed to import genesis key for P-chain: %w in %q", err, nodeName)
		}
		lc.pchainPreFundedAddr = pAddr
		if lc.pchainPreFundedAddr != expectedPchainFundedAddr {
			return fmt.Errorf("unexpected P-chain funded address %q (expected %q)", lc.pchainPreFundedAddr, expectedPchainFundedAddr)
		}
		pBalance, err := cli.PChainAPI().GetBalance(pAddr)
		if err != nil {
			return fmt.Errorf("failed to get P-chain balance: %w in %q", err, nodeName)
		}
		color.Cyan("funded P-chain: address %q, balance %d $AVAX in %q", pAddr, pBalance.Balance, nodeName)

		cAddr, err := cli.CChainAPI().ImportKey(userPass, genesisPrivKey)
		if err != nil {
			return fmt.Errorf("failed to import genesis key for P-chain: %w in %q", err, nodeName)
		}
		lc.cchainPreFundedAddr = cAddr
		if lc.cchainPreFundedAddr != expectedCchainFundedAddr {
			return fmt.Errorf("unexpected C-chain funded address %q (expected %q)", lc.cchainPreFundedAddr, expectedCchainFundedAddr)
		}

		// TODO: not working?
		// ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
		// cBalance, err := cli.CChainEthAPI().BalanceAt(ctx, common.HexToAddress(cAddr), nil)
		// cancel()
		// if err != nil {
		// 	return fmt.Errorf("failed to get C-chain balance: %w in %q", err, name)
		// }
		// color.Cyan("funded C-chain address: %q, balance %d $AVAX in %q", cAddr, cBalance.Int64(), name)
	}

	return nil
}

func (lc *localNetwork) createAddresses() error {
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		color.Blue("creating X-chain address")
		xAddr, err := cli.XChainAPI().CreateAddress(userPass)
		if err != nil {
			return fmt.Errorf("failed to create X-chain address: %w in %q", err, nodeName)
		}
		lc.xchainFundedAddrs[nodeName] = xAddr

		color.Blue("creating P-chain address")
		pAddr, err := cli.PChainAPI().CreateAddress(userPass)
		if err != nil {
			return fmt.Errorf("failed to create P-chain address: %w in %q", err, nodeName)
		}
		lc.pchainFundedAddrs[nodeName] = pAddr
	}
	color.Blue("created addresses: X-chain %q, P-chain %q", lc.xchainFundedAddrs, lc.pchainFundedAddrs)
	return nil
}

func (lc *localNetwork) checkXChainAddress(nodeName string, addr string) error {
	color.Blue("checking X-chain address %q in %q", addr, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	xcli := cli.XChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		addrs, err := xcli.ListAddresses(userPass)
		if err != nil {
			color.Yellow("failed to list addresses %v in %q", err, nodeName)
			continue
		}
		found := false
		for _, a := range addrs {
			if a == addr {
				found = true
				break
			}
		}
		if !found {
			color.Yellow("failed to find address %q in %q (got %q)", addr, nodeName, addrs)
			continue
		}

		color.Cyan("confirmed address %q in %q (got %q)", addr, nodeName, addrs)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) checkPChainAddress(nodeName string, addr string) error {
	color.Blue("checking P-chain address %q in %q", addr, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	pcli := cli.PChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		addrs, err := pcli.ListAddresses(userPass)
		if err != nil {
			color.Yellow("failed to list addresses %v in %q", err, nodeName)
			continue
		}
		found := false
		for _, a := range addrs {
			if a == addr {
				found = true
				break
			}
		}
		if !found {
			color.Yellow("failed to find address %q in %q (got %q)", err, nodeName, addrs)
			continue
		}

		color.Cyan("confirmed address %q in %q (got %q)", addr, nodeName, addrs)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) transferFunds() error {
	color.Blue("transfering funds...")

	return nil
}

func (lc *localNetwork) checkXChainTx(nodeName string, txID ids.ID) error {
	color.Blue("checking X-chain tx %q in %q", txID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	xcli := cli.XChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		status, err := xcli.GetTxStatus(txID)
		if err != nil {
			color.Yellow("failed to get tx status %v in %q", err, nodeName)
			continue
		}
		if status != choices.Accepted {
			color.Yellow("subnet tx %s status %q in %q", txID, status, nodeName)
			continue
		}

		color.Cyan("confirmed tx %q %q in %q", txID, status, nodeName)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) checkPChainTx(nodeName string, txID ids.ID) error {
	color.Blue("checking P-chain tx %q in %q", txID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	pcli := cli.PChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		status, err := pcli.GetTxStatus(txID, true)
		if err != nil {
			color.Yellow("failed to get tx status %v in %q", err, nodeName)
			continue
		}
		if status.Status != platformvm.Committed {
			color.Yellow("subnet tx %s status %q in %q", txID, status.Status, nodeName)
			continue
		}

		color.Cyan("confirmed tx %q %q in %q", txID, status.Status, nodeName)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) createSubnet() error {
	color.Blue("creating subnet...")
	nodeName := lc.nodeNames[0]
	cli := lc.apiClis[nodeName]
	subnetTxID, err := cli.PChainAPI().CreateSubnet(
		userPass,
		[]string{lc.pchainPreFundedAddr}, // from
		lc.pchainPreFundedAddr,           // changeAddr
		[]string{lc.pchainPreFundedAddr}, // controlKeys
		1,                                // threshold
	)
	if err != nil {
		return fmt.Errorf("failed to create subnet: %w in %q", err, nodeName)
	}
	lc.subnetTxID = subnetTxID
	if lc.subnetTxID.String() != expectedSubnetTxID {
		return fmt.Errorf("unexpected subnet tx ID %q (expected %q)", lc.subnetTxID, expectedSubnetTxID)
	}

	color.Blue("created subnet %q in %q", subnetTxID, nodeName)
	return nil
}

func (lc *localNetwork) checkSubnet(nodeName string) error {
	color.Blue("checking subnet exists %q in %q", lc.subnetTxID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	pcli := cli.PChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		subnets, err := pcli.GetSubnets([]ids.ID{})
		if err != nil {
			color.Yellow("failed to get subnets %v in %q", err, nodeName)
			continue
		}

		found := false
		for _, sub := range subnets {
			if sub.ID == lc.subnetTxID {
				found = true
				color.Cyan("%q returned expected subnet ID %q", nodeName, sub.ID)
				break
			}
			color.Yellow("%q returned unexpected subnet ID %q", nodeName, sub.ID)
		}
		if !found {
			color.Yellow("%q does not have subnet %q", nodeName, lc.subnetTxID)
			continue
		}

		color.Cyan("confirmed subnet exists %q in %q", lc.subnetTxID, nodeName)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) addSubnetValidators() error {
	color.Blue("adding subnet validator...")
	for nodeName, cli := range lc.apiClis {
		valTxID, err := cli.PChainAPI().AddSubnetValidator(
			userPass,
			[]string{lc.pchainPreFundedAddr}, // from
			lc.pchainPreFundedAddr,           // changeAddr
			lc.subnetTxID.String(),           // subnetID
			lc.nodeIDs[nodeName],             // nodeID
			validatorWeight,                  // stakeAmount
			uint64(time.Now().Add(validatorStartDiff).Unix()), // startTime
			uint64(time.Now().Add(validatorEndDiff).Unix()),   // endTime
		)
		if err != nil {
			return fmt.Errorf("failed to add subnet validator: %w in %q", err, nodeName)
		}
		if err := lc.checkPChainTx(nodeName, valTxID); err != nil {
			return err
		}
		color.Cyan("added subnet validator %q in %q", valTxID, nodeName)
	}
	return nil
}

func (lc *localNetwork) createBlockchain() error {
	vmGenesis, err := ioutil.ReadFile(lc.vmGenesisPath)
	if err != nil {
		return fmt.Errorf("failed to read genesis file (%s): %w", lc.vmGenesisPath, err)
	}

	color.Blue("creating blockchain with vm name %q and ID %q...", lc.vmName, lc.vmID)
	for name, cli := range lc.apiClis {
		blkChainTxID, err := cli.PChainAPI().CreateBlockchain(
			userPass,
			[]string{lc.pchainPreFundedAddr}, // from
			lc.pchainPreFundedAddr,           // changeAddr
			lc.subnetTxID,                    // subnetID
			lc.vmID,                          // vmID
			[]string{},                       // fxIDs
			lc.vmName,                        // name
			vmGenesis,                        // genesisData
		)
		if err != nil {
			return fmt.Errorf("failed to create blockchain: %w in %q", err, name)
		}
		lc.blkChainTxID = blkChainTxID
		color.Blue("created blockchain %q in %q", blkChainTxID, name)
		break
	}
	return nil
}

func (lc *localNetwork) checkBlockchain(nodeName string) error {
	color.Blue("checking blockchain exists %q in %q", lc.blkChainTxID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	pcli := cli.PChainAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		blockchains, err := pcli.GetBlockchains()
		if err != nil {
			color.Yellow("failed to get blockchains %v in %q", err, nodeName)
			continue
		}
		blockchainID := ids.Empty
		for _, blockchain := range blockchains {
			if blockchain.SubnetID == lc.subnetTxID {
				blockchainID = blockchain.ID
				break
			}
		}
		if blockchainID == ids.Empty {
			color.Yellow("failed to get blockchain ID in %q", nodeName)
			continue
		}
		if lc.blkChainTxID != blockchainID {
			color.Yellow("unexpected blockchain ID %q in %q (expected %q)", nodeName, lc.blkChainTxID)
			continue
		}

		status, err := pcli.GetBlockchainStatus(blockchainID.String())
		if err != nil {
			color.Yellow("failed to get blockchain status %v in %q", err, nodeName)
			continue
		}
		if status != platformvm.Validating {
			color.Yellow("blockchain status %q in %q, retrying", status, nodeName)
			continue
		}

		color.Cyan("confirmed blockchain exists and status %q in %q", status, nodeName)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) checkBlockchainBootstrapped(nodeName string) error {
	color.Blue("checking blockchain bootstrapped %q in %q", lc.blkChainTxID, nodeName)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	icli := cli.InfoAPI()

	ctx, cancel := context.WithTimeout(context.Background(), txConfirmWait)
	defer cancel()
	for ctx.Err() == nil {
		select {
		case <-lc.stopc:
			return errAborted
		case <-time.After(checkInterval):
		}

		bootstrapped, err := icli.IsBootstrapped(lc.blkChainTxID.String())
		if err != nil {
			color.Yellow("failed to check blockchain bootstrapped %v in %q", err, nodeName)
			continue
		}
		if !bootstrapped {
			color.Yellow("blockchain %q in %q not bootstrapped yet", lc.blkChainTxID, nodeName)
			continue
		}

		color.Cyan("confirmed blockchain bootstrapped %q in %q", lc.blkChainTxID, nodeName)
		return nil
	}
	return ctx.Err()
}

func (lc *localNetwork) getURIs() []string {
	uris := make([]string, 0, len(lc.uris))
	for _, u := range lc.uris {
		uris = append(uris, u)
	}
	sort.Strings(uris)
	return uris
}

func (lc *localNetwork) writeOutput() error {
	pid := os.Getpid()
	color.Blue("writing output %q with PID %d", lc.outputPath, pid)
	ci := ClusterInfo{
		URIs:     lc.getURIs(),
		Endpoint: fmt.Sprintf("/ext/bc/%s", lc.blkChainTxID),
		PID:      pid,
		LogsDir:  lc.logsDir,
	}
	err := ci.Save(lc.outputPath)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(lc.outputPath)
	if err != nil {
		return err
	}
	fmt.Printf("\ncat %s:\n\n%s\n", lc.outputPath, string(b))
	return nil
}

func (lc *localNetwork) shutdown() {
	close(lc.stopc)
	serr := lc.nw.Stop(context.Background())
	<-lc.donec
	color.Red("terminated network (error %v)", serr)
}
