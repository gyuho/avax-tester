package runner

import (
	"context"
	"fmt"
	"time"

	avago_api "github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/coreth/ethclient"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fatih/color"
)

// need to hard-code user-pass in order to
// determine subnet ID for whitelisting
var userPass = avago_api.UserPass{
	Username: "testuser",
	Password: "testinsecurerandomvmavax",
}

const (
	rawEwoqPk = "ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	ewoqPk    = "PrivateKey-" + rawEwoqPk

	// expected response from "ImportKey" based on hard-coded "ewoqPk"
	expectedXchainEwoqAddr = "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedPchainEwoqAddr = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedCchainEwoqAddr = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
)

var ewoqPrivateKey crypto.PrivateKey

func init() {
	skBytes, err := formatting.Decode(formatting.CB58, rawEwoqPk)
	if err != nil {
		panic(err)
	}
	factory := &crypto.FactorySECP256K1R{}
	ewoqPrivateKey, err = factory.ToPrivateKey(skBytes)
	if err != nil {
		panic(err)
	}
	color.Blue("loaded ewoq private key %q", ewoqPrivateKey.PublicKey().Address().Hex())
}

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

func (lc *localNetwork) fundWithEwoq() error {
	color.Blue("importing genesis key and funds to the user in all nodes...")
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		xAddr, err := cli.XChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import genesis key for X-chain: %w in %q", err, nodeName)
		}
		lc.xchainPreFundedAddr = xAddr
		xBalance, err := cli.XChainAPI().GetBalance(xAddr, "AVAX", false)
		if err != nil {
			return fmt.Errorf("failed to get X-chain balance: %w in %q", err, nodeName)
		}
		if lc.xchainPreFundedAddr != expectedXchainEwoqAddr {
			return fmt.Errorf("unexpected X-chain funded address %q (expected %q)", lc.xchainPreFundedAddr, expectedXchainEwoqAddr)
		}
		color.Cyan("funded X-chain: address %q, balance $AVAX %d in %q", xAddr, xBalance.Balance, nodeName)

		pAddr, err := cli.PChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import genesis key for P-chain: %w in %q", err, nodeName)
		}
		lc.pchainPreFundedAddr = pAddr
		if lc.pchainPreFundedAddr != expectedPchainEwoqAddr {
			return fmt.Errorf("unexpected P-chain funded address %q (expected %q)", lc.pchainPreFundedAddr, expectedPchainEwoqAddr)
		}
		pBalance, err := cli.PChainAPI().GetBalance(pAddr)
		if err != nil {
			return fmt.Errorf("failed to get P-chain balance: %w in %q", err, nodeName)
		}
		color.Cyan("funded P-chain: address %q, balance $AVAX %d in %q", pAddr, pBalance.Balance, nodeName)

		cAddr, err := cli.CChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import genesis key for P-chain: %w in %q", err, nodeName)
		}
		lc.cchainPreFundedAddr = cAddr
		if lc.cchainPreFundedAddr != expectedCchainEwoqAddr {
			return fmt.Errorf("unexpected C-chain funded address %q (expected %q)", lc.cchainPreFundedAddr, expectedCchainEwoqAddr)
		}

		// TODO: timeout
		// failed to get tx status problem while making JSON RPC POST request to http://localhost:53859/ext/P: Post "http://localhost:53859/ext/P": context deadline exceeded
		// cBalance, err := cli.CChainEthAPI().BalanceAt(ctx, common.HexToAddress(cAddr), nil)

		ethCli, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/C/rpc", lc.uris[nodeName]))
		if err != nil {
			return fmt.Errorf("failed to dial %q (%w)", nodeName, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cBalance, err := ethCli.BalanceAt(ctx, common.HexToAddress(cAddr), nil)
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get C-chain balance: %w in %q", err, nodeName)
		}
		color.Cyan("funded C-chain address: %q, balance $AVAX %d in %q", cAddr, cBalance.Int64(), nodeName)
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
		lc.xchainAddrs[nodeName] = xAddr

		color.Blue("creating P-chain address")
		pAddr, err := cli.PChainAPI().CreateAddress(userPass)
		if err != nil {
			return fmt.Errorf("failed to create P-chain address: %w in %q", err, nodeName)
		}
		lc.pchainAddrs[nodeName] = pAddr
	}
	color.Blue("created addresses: X-chain %q, P-chain %q", lc.xchainAddrs, lc.pchainAddrs)
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

func generateKey() (*crypto.PrivateKeySECP256K1R, error) {
	factory := &crypto.FactorySECP256K1R{}
	pk, err := factory.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	return pk.(*crypto.PrivateKeySECP256K1R), nil
}
