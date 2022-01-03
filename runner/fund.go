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

var ewoqPrivateKey *crypto.PrivateKeySECP256K1R

func init() {
	skBytes, err := formatting.Decode(formatting.CB58, rawEwoqPk)
	if err != nil {
		panic(err)
	}
	factory := &crypto.FactorySECP256K1R{}
	rpk, err := factory.ToPrivateKey(skBytes)
	if err != nil {
		panic(err)
	}
	ewoqPrivateKey = rpk.(*crypto.PrivateKeySECP256K1R)
	color.Blue("loaded ewoq private key %q", ewoqPrivateKey.PublicKey().Address().Hex())
}

func (lc *localNetwork) createNewWallets() error {
	lc.wallets = make([]*wallet, 5)
	for i := range lc.wallets {
		factory := &crypto.FactorySECP256K1R{}
		rpk, err := factory.NewPrivateKey()
		if err != nil {
			return err
		}
		spk := rpk.(*crypto.PrivateKeySECP256K1R)
		lc.wallets[i] = newWallet(randutil.String(10), spk)
	}
	return nil
}

func (lc *localNetwork) importEwoqWallet() error {
	color.Blue("importing ewoq and funds to the user in all nodes...")
	lc.ewoqWallet = newWallet("ewoq", ewoqPrivateKey)
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		xAddr, err := cli.XChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import ewoq for X-Chain: %w in %q", err, nodeName)
		}
		lc.ewoqWallet.xChainAddr = xAddr
		if lc.ewoqWallet.xChainAddr != expectedXChainEwoqAddr {
			return fmt.Errorf("unexpected X-Chain funded address %q (expected %q)", lc.ewoqWallet.xChainAddr, expectedXChainEwoqAddr)
		}
		color.Cyan("imported ewoq to X-Chain with address %q in %q", xAddr, nodeName)

		pAddr, err := cli.PChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import ewoq for P-chain: %w in %q", err, nodeName)
		}
		lc.ewoqWallet.pChainAddr = pAddr
		if lc.ewoqWallet.pChainAddr != expectedPChainEwoqAddr {
			return fmt.Errorf("unexpected P-chain funded address %q (expected %q)", lc.ewoqWallet.pChainAddr, expectedPChainEwoqAddr)
		}
		color.Cyan("imported ewoq to P-chain with address %q in %q", pAddr, nodeName)

		cAddr, err := cli.CChainAPI().ImportKey(userPass, ewoqPk)
		if err != nil {
			return fmt.Errorf("failed to import ewoq for P-chain: %w in %q", err, nodeName)
		}
		lc.ewoqWallet.cChainAddr = cAddr
		if lc.ewoqWallet.cChainAddr != expectedCChainEwoqAddr {
			return fmt.Errorf("unexpected C-chain funded address %q (expected %q)", lc.ewoqWallet.cChainAddr, expectedCChainEwoqAddr)
		}
		color.Cyan("imported ewoq to C-chain with address %q in %q", cAddr, nodeName)
	}
	return nil
}

func (lc *localNetwork) importNewWallets() error {
	color.Blue("importing new wallets to the user in all nodes...")
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]
		for _, w := range lc.wallets {
			xAddr, err := cli.XChainAPI().ImportKey(userPass, w.privKeyEncoded)
			if err != nil {
				return fmt.Errorf("failed to import wallet %q for X-Chain: %w in %q", w.name, err, nodeName)
			}
			if w.xChainAddr != xAddr {
				return fmt.Errorf("unexpected X-Chain funded address %q (expected %q)", xAddr, w.xChainAddr)
			}
			color.Cyan("imported wallet %q to X-Chain with address %q in %q", w.name, xAddr, nodeName)

			pAddr, err := cli.PChainAPI().ImportKey(userPass, w.privKeyEncoded)
			if err != nil {
				return fmt.Errorf("failed to import wallet %q for P-chain: %w in %q", w.name, err, nodeName)
			}
			if w.pChainAddr != pAddr {
				return fmt.Errorf("unexpected P-chain funded address %q (expected %q)", pAddr, w.pChainAddr)
			}
			color.Cyan("imported wallet %q to P-chain with address %q in %q", w.name, pAddr, nodeName)

			cAddr, err := cli.CChainAPI().ImportKey(userPass, w.privKeyEncoded)
			if err != nil {
				return fmt.Errorf("failed to import wallet %q for P-chain: %w in %q", w.name, err, nodeName)
			}
			if w.commonAddr.String() != cAddr {
				return fmt.Errorf("unexpected C-chain funded address %q (expected %q)", cAddr, w.commonAddr)
			}
			color.Cyan("imported wallet %q to C-chain with address %q in %q", w.name, cAddr, nodeName)
		}
	}
	return nil
}

func (lc *localNetwork) fetchBalanceEwoq() error {
	color.Blue("importing ewoq and funds to the user in all nodes...")
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		xBalance, err := cli.XChainAPI().GetBalance(lc.ewoqWallet.xChainAddr, "AVAX", false)
		if err != nil {
			return fmt.Errorf("failed to get X-Chain balance: %w in %q", err, nodeName)
		}
		lc.ewoqWallet.xChainBal = uint64(xBalance.Balance)
		color.Cyan("ewoq X-Chain balance $AVAX %d at address %q in %q", lc.ewoqWallet.xChainBal, lc.ewoqWallet.xChainAddr, nodeName)

		pBalance, err := cli.PChainAPI().GetBalance(lc.ewoqWallet.pChainAddr)
		if err != nil {
			return fmt.Errorf("failed to get P-chain balance: %w in %q", err, nodeName)
		}
		lc.ewoqWallet.pChainBal = uint64(pBalance.Balance)
		color.Cyan("ewoq P-chain balance $AVAX %d at address %q in %q", lc.ewoqWallet.pChainBal, lc.ewoqWallet.pChainAddr, nodeName)

		// TODO: timeout
		// failed to get tx status problem while making JSON RPC POST request to http://localhost:53859/ext/P: Post "http://localhost:53859/ext/P": context deadline exceeded
		// cli.CChainEthAPI().BalanceAt(ctx, ...
		ethCli, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/C/rpc", lc.uris[nodeName]))
		if err != nil {
			return fmt.Errorf("failed to dial %q (%w)", nodeName, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cBalance, err := ethCli.BalanceAt(ctx, lc.ewoqWallet.commonAddr, nil)
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get C-chain balance: %w in %q", err, nodeName)
		}
		lc.ewoqWallet.cChainBal = cBalance.Uint64()
		color.Cyan("ewoq C-chain balance $AVAX %d at address %q in %q", lc.ewoqWallet.cChainBal, lc.ewoqWallet.cChainAddr, nodeName)
	}
	return nil
}

func (lc *localNetwork) fetchBalanceWallets() error {
	for _, nodeName := range lc.nodeNames {
		cli := lc.apiClis[nodeName]

		color.Blue("fetching wallet balances in %q", nodeName)
		ethCli, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/C/rpc", lc.uris[nodeName]))
		if err != nil {
			return fmt.Errorf("failed to dial %q (%w)", nodeName, err)
		}
		for _, w := range lc.wallets {
			xBalance, err := cli.XChainAPI().GetBalance(w.xChainAddr, "AVAX", false)
			if err != nil {
				return fmt.Errorf("failed to get X-Chain balance: %w in %q", err, nodeName)
			}
			w.xChainBal = uint64(xBalance.Balance)
			color.Cyan("%q X-Chain balance $AVAX %d at address %q in %q", w.name, w.xChainBal, w.xChainAddr, nodeName)

			pBalance, err := cli.PChainAPI().GetBalance(w.pChainAddr)
			if err != nil {
				return fmt.Errorf("failed to get P-chain balance: %w in %q", err, nodeName)
			}
			w.pChainBal = uint64(pBalance.Balance)
			color.Cyan("%q P-chain balance $AVAX %d at address %q in %q", w.name, w.pChainBal, w.pChainAddr, nodeName)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			// or "common.HexToAddress(lc.ewoqWallet.cChainAddr)"
			cBalance, err := ethCli.BalanceAt(ctx, w.commonAddr, nil)
			cancel()
			if err != nil {
				return fmt.Errorf("failed to get C-chain balance: %w in %q", err, nodeName)
			}
			w.cChainBal = cBalance.Uint64()
			color.Cyan("%q C-chain balance $AVAX %d at address %q in %q", w.name, w.cChainBal, w.cChainAddr, nodeName)
		}
	}
	return nil
}

// withdraw from ewoq X-Chain to a new wallet X-Chain
func (lc *localNetwork) withdrawEwoqXChain(nodeName string) error {
	color.Blue("withdrawing X-Chain funds from ewoq %q to %q %q in %q",
		lc.ewoqWallet.xChainAddr,
		lc.wallets[0].name,
		lc.wallets[0].xChainAddr,
		nodeName,
	)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	txID, err := cli.XChainWalletAPI().Send(
		userPass,
		[]string{lc.ewoqWallet.xChainAddr}, // from
		"",                                 // changeAddr
		100000,                             // amount
		"AVAX",                             // asset
		lc.wallets[0].xChainAddr,           // to
		"hi!",                              // message
	)
	if err != nil {
		return err
	}
	return lc.checkXChainTx(nodeName, txID)
}

// import from the funded new wallet X-Chain above to its P-Chain
func (lc *localNetwork) importXtoPChain(nodeName string) error {
	color.Blue("withdrawing funds from %q %q to %q in %q",
		lc.wallets[0].name,
		lc.wallets[0].xChainAddr,
		lc.wallets[0].pChainAddr,
		nodeName,
	)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}

	// avm.getAssetDescription to get AVAX asset ID
	avaxDesc, err := cli.XChainAPI().GetAssetDescription("AVAX")
	if err != nil {
		return fmt.Errorf("%q failed to get AVAX asset ID %w", nodeName, err)
	}
	// constants.PlatformChainID for P-Chain ID
	xChainID, err := cli.InfoAPI().GetBlockchainID("X")
	if err != nil {
		return fmt.Errorf("%q failed to get blockchain ID %w", nodeName, err)
	}

	// "from" empty, only transfers from ewoq
	// "from" with wallet[0] has "100000" and amount "50000"
	// Error: "node2" failed to export AVAX asset want to spend 1050000 of asset BUuypiq2wyuLMvyhzFXcPyxPMCgSp7eeDohhQRqTChoBjKziC but only have 100000

	color.Blue("exporting X-Chain asset %q to %q", avaxDesc.AssetID, lc.wallets[0].pChainAddr)
	txID, err := cli.XChainAPI().Export(
		userPass,
		[]string{lc.wallets[0].xChainAddr}, // from
		"",                                 // changeAddr
		50000,                              // amount
		lc.wallets[0].pChainAddr,           // to
		avaxDesc.AssetID.String(),          // assetID
	)
	if err != nil {
		return fmt.Errorf("%q failed to export AVAX asset %w", nodeName, err)
	}
	if err := lc.checkXChainTx(nodeName, txID); err != nil {
		return err
	}

	color.Blue("importing X-Chain asset %q to %q", avaxDesc.AssetID, lc.wallets[0].pChainAddr)
	txID, err = cli.PChainAPI().ImportAVAX(
		userPass,
		// nil,                      // from
		[]string{lc.wallets[0].xChainAddr}, // from
		"",                                 // changeAddr
		lc.wallets[0].pChainAddr,           // to
		xChainID.String(),                  // sourceChain
	)
	if err != nil {
		return err
	}
	return lc.checkPChainTx(nodeName, txID)
}

func (lc *localNetwork) withdrawEwoqCChain(nodeName string) error {
	color.Blue("withdrawing C-Chain funds from ewoq %q to wallet %q in %q",
		lc.ewoqWallet.cChainAddr,
		lc.wallets[0].cChainAddr,
		nodeName,
	)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	_ = cli

	return nil
}

func (lc *localNetwork) checkXChainTx(nodeName string, txID ids.ID) error {
	color.Blue("checking X-Chain tx %q in %q", txID, nodeName)
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
			color.Yellow("tx %s status %q in %q", txID, status, nodeName)
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
			color.Yellow("tx %s status %q in %q", txID, status.Status, nodeName)
			continue
		}

		color.Cyan("confirmed tx %q %q in %q", txID, status.Status, nodeName)
		return nil
	}
	return ctx.Err()
}
