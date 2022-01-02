package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/coreth/ethclient"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/fatih/color"
	"github.com/gyuho/avax-tester/pkg/randutil"
)

const (
	rawEwoqPk = "ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	ewoqPk    = "PrivateKey-" + rawEwoqPk

	// expected response from "ImportKey" based on hard-coded "ewoqPk"
	expectedXChainEwoqAddr = "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedPChainEwoqAddr = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
	expectedCChainEwoqAddr = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
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

func (lc *localNetwork) importEwoq() error {
	color.Blue("importing ewoq and funds to the user in all nodes...")
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		xAddr, err := cli.XChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import ewoq for X-chain: %w in %q", err, nodeName)
		}
		lc.ewoqXChainAddr = xAddr
		if lc.ewoqXChainAddr != expectedXChainEwoqAddr {
			return fmt.Errorf("unexpected X-chain funded address %q (expected %q)", lc.ewoqXChainAddr, expectedXChainEwoqAddr)
		}
		color.Cyan("imported ewoq to X-chain with address %q in %q", xAddr, nodeName)

		pAddr, err := cli.PChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import ewoq for P-chain: %w in %q", err, nodeName)
		}
		lc.ewoqPChainAddr = pAddr
		if lc.ewoqPChainAddr != expectedPChainEwoqAddr {
			return fmt.Errorf("unexpected P-chain funded address %q (expected %q)", lc.ewoqPChainAddr, expectedPChainEwoqAddr)
		}
		color.Cyan("imported ewoq to P-chain with address %q in %q", pAddr, nodeName)

		cAddr, err := cli.CChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import ewoq for P-chain: %w in %q", err, nodeName)
		}
		lc.ewoqCChainAddr = cAddr
		if lc.ewoqCChainAddr != expectedCChainEwoqAddr {
			return fmt.Errorf("unexpected C-chain funded address %q (expected %q)", lc.ewoqCChainAddr, expectedCChainEwoqAddr)
		}
		color.Cyan("imported ewoq to C-chain with address %q in %q", cAddr, nodeName)
	}
	return nil
}

func (lc *localNetwork) fetchEwoqBalances() error {
	color.Blue("importing ewoq and funds to the user in all nodes...")
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		xBalance, err := cli.XChainAPI().GetBalance(lc.ewoqXChainAddr, "AVAX", false)
		if err != nil {
			return fmt.Errorf("failed to get X-chain balance: %w in %q", err, nodeName)
		}
		lc.ewoqXChainBal = uint64(xBalance.Balance)
		color.Cyan("ewoq X-chain balance $AVAX %d at address %q in %q", lc.ewoqXChainBal, lc.ewoqXChainAddr, nodeName)

		pBalance, err := cli.PChainAPI().GetBalance(lc.ewoqPChainAddr)
		if err != nil {
			return fmt.Errorf("failed to get P-chain balance: %w in %q", err, nodeName)
		}
		lc.ewoqPChainBal = uint64(pBalance.Balance)
		color.Cyan("ewoq P-chain balance $AVAX %d at address %q in %q", lc.ewoqPChainBal, lc.ewoqPChainAddr, nodeName)

		// TODO: timeout
		// failed to get tx status problem while making JSON RPC POST request to http://localhost:53859/ext/P: Post "http://localhost:53859/ext/P": context deadline exceeded
		// cli.CChainEthAPI().BalanceAt(ctx, common.HexToAddress...
		ethCli, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/C/rpc", lc.uris[nodeName]))
		if err != nil {
			return fmt.Errorf("failed to dial %q (%w)", nodeName, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cBalance, err := ethCli.BalanceAt(ctx, common.HexToAddress(lc.ewoqCChainAddr), nil)
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get C-chain balance: %w in %q", err, nodeName)
		}
		lc.ewoqCChainBal = cBalance.Uint64()
		color.Cyan("ewoq C-chain balance $AVAX %d at address %q in %q", lc.ewoqCChainBal, lc.ewoqCChainAddr, nodeName)
	}
	return nil
}

func (lc *localNetwork) createWallets() error {
	lc.wallets = []*wallet{
		newWallet(randutil.String(10)),
		newWallet(randutil.String(10)),
		newWallet(randutil.String(10)),
		newWallet(randutil.String(10)),
		newWallet(randutil.String(10)),
	}
	return nil
}

func (lc *localNetwork) fetchWalletBalances() error {
	for _, nodeName := range lc.nodeNames {
		color.Blue("fetching wallet balances in %q", nodeName)
		ethCli, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/C/rpc", lc.uris[nodeName]))
		if err != nil {
			return fmt.Errorf("failed to dial %q (%w)", nodeName, err)
		}

		for i, w := range lc.wallets {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cBalance, err := ethCli.BalanceAt(ctx, w.addr, nil)
			cancel()
			if err != nil {
				return fmt.Errorf("failed to get C-chain balance: %w in %q", err, nodeName)
			}
			lc.wallets[i].balance = cBalance.Uint64()
			color.Cyan("%q at %s balance: %d", w.name, w.addr, lc.wallets[i].balance)
		}
	}
	return nil
}

func newWallet(name string) *wallet {
	factory := &crypto.FactorySECP256K1R{}
	rpk, err := factory.NewPrivateKey()
	if err != nil {
		panic(err)
	}
	spk := rpk.(*crypto.PrivateKeySECP256K1R)

	pk := spk.ToECDSA()
	addr := ethcrypto.PubkeyToAddress(pk.PublicKey)

	return &wallet{
		name:    name,
		spk:     spk,
		addr:    addr,
		balance: 0,
	}
}

type wallet struct {
	name    string
	spk     *crypto.PrivateKeySECP256K1R
	addr    common.Address
	balance uint64
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
