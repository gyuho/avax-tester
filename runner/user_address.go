package runner

import (
	"context"
	"fmt"
	"time"

	avago_api "github.com/ava-labs/avalanchego/api"
	"github.com/fatih/color"
)

// need to hard-code user-pass in order to
// determine subnet ID for whitelisting
var userPass = avago_api.UserPass{
	Username: "testuser",
	Password: "testinsecurerandomvmavax",
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

func (lc *localNetwork) createSecondaryAddresses() error {
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		color.Blue("creating X-Chain address")
		xAddr, err := cli.XChainAPI().CreateAddress(userPass)
		if err != nil {
			return fmt.Errorf("failed to create X-Chain address: %w in %q", err, nodeName)
		}
		lc.xChainSecondaryAddrs[nodeName] = xAddr

		color.Blue("creating P-chain address")
		pAddr, err := cli.PChainAPI().CreateAddress(userPass)
		if err != nil {
			return fmt.Errorf("failed to create P-chain address: %w in %q", err, nodeName)
		}
		lc.pChainSecondaryAddrs[nodeName] = pAddr
	}
	color.Blue("created addresses: X-Chain %q, P-chain %q", lc.xChainSecondaryAddrs, lc.pChainSecondaryAddrs)
	return nil
}

func (lc *localNetwork) checkXChainAddress(nodeName string, addr string) error {
	color.Blue("checking X-Chain address %q in %q", addr, nodeName)
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
