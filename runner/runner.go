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
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
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

	xChainSecondaryAddrs map[string]string
	pChainSecondaryAddrs map[string]string

	ewoqWallet *wallet
	wallets    []*wallet

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

		xChainSecondaryAddrs: make(map[string]string),
		pChainSecondaryAddrs: make(map[string]string),

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
	if err := lc.createSecondaryAddresses(); err != nil {
		lc.errc <- err
		return
	}
	for name, addr := range lc.xChainSecondaryAddrs {
		if err := lc.checkXChainAddress(name, addr); err != nil {
			lc.errc <- err
			return
		}
	}
	for name, addr := range lc.pChainSecondaryAddrs {
		if err := lc.checkPChainAddress(name, addr); err != nil {
			lc.errc <- err
			return
		}
	}

	if err := lc.importEwoq(); err != nil {
		lc.errc <- err
		return
	}
	for _, name := range lc.nodeNames {
		if err := lc.checkXChainAddress(name, lc.ewoqWallet.xChainAddr); err != nil {
			lc.errc <- err
			return
		}
		if err := lc.checkPChainAddress(name, lc.ewoqWallet.pChainAddr); err != nil {
			lc.errc <- err
			return
		}
	}
	if err := lc.fetchBalanceEwoq(); err != nil {
		lc.errc <- err
		return
	}
	if err := lc.createWallets(); err != nil {
		lc.errc <- err
		return
	}
	if err := lc.fetchBalanceWallets(); err != nil {
		lc.errc <- err
		return
	}

	if err := lc.withdrawEwoqXChain(lc.nodeNames[0]); err != nil {
		lc.errc <- err
		return
	}
	// if err := lc.withdrawEwoqPChain(); err != nil {
	// 	lc.errc <- err
	// 	return
	// }
	// if err := lc.withdrawEwoqCChain(); err != nil {
	// 	lc.errc <- err
	// 	return
	// }

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

const healthyWait = 2 * time.Minute

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

func (lc *localNetwork) writeOutput() error {
	pid := os.Getpid()
	color.Blue("writing output %q with PID %d", lc.outputPath, pid)
	wallets := make([]Wallet, len(lc.wallets))
	for i := range wallets {
		wallets[i] = Wallet{
			Name:            lc.wallets[i].name,
			PrivateKeyBytes: lc.wallets[i].spk.Bytes(),
			CommonAddress:   lc.wallets[i].commonAddr.String(),
			XChainAddress:   lc.wallets[i].xChainAddr,
			XChainBalance:   lc.wallets[i].xChainBal,
			PChainAddress:   lc.wallets[i].pChainAddr,
			PChainBalance:   lc.wallets[i].pChainBal,
			CChainAddress:   lc.wallets[i].cChainAddr,
			CChainBalance:   lc.wallets[i].cChainBal,
		}
	}
	ci := ClusterInfo{
		URIs:     lc.getURIs(),
		Endpoint: fmt.Sprintf("/ext/bc/%s", lc.blkChainTxID),
		PID:      pid,
		LogsDir:  lc.logsDir,

		XChainSecondaryAddresses: lc.xChainSecondaryAddrs,
		PChainSecondaryAddresses: lc.pChainSecondaryAddrs,

		EwoqWallet: Wallet{
			Name:            lc.ewoqWallet.name,
			PrivateKeyBytes: lc.ewoqWallet.spk.Bytes(),
			CommonAddress:   lc.ewoqWallet.commonAddr.String(),
			XChainAddress:   lc.ewoqWallet.xChainAddr,
			XChainBalance:   lc.ewoqWallet.xChainBal,
			PChainAddress:   lc.ewoqWallet.pChainAddr,
			PChainBalance:   lc.ewoqWallet.pChainBal,
			CChainAddress:   lc.ewoqWallet.cChainAddr,
			CChainBalance:   lc.ewoqWallet.cChainBal,
		},
		Wallets: wallets,
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

func (lc *localNetwork) getURIs() []string {
	uris := make([]string, 0, len(lc.uris))
	for _, u := range lc.uris {
		uris = append(uris, u)
	}
	sort.Strings(uris)
	return uris
}
