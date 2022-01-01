package runner

import (
	"context"
	"fmt"
	"time"

	avago_api "github.com/ava-labs/avalanchego/api"
	"github.com/fatih/color"
)

var (
	// need to hard-code user-pass in order to
	// determine subnet ID for whitelisting
	userPass = avago_api.UserPass{
		Username: "test",
		Password: "vmsrkewl",
	}
)

const (
	// expected response from "ImportKey"
	// based on hard-coded "userPass" and "genesisPrivKey"
	expectedXchainFundedAddr = "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedPchainFundedAddr = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedCchainFundedAddr = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
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
