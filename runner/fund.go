package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
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
func (lc *localNetwork) withdrawEwoqXChainToWallet(nodeName string, to *wallet) error {
	color.Blue("withdrawing X-Chain funds from ewoq %q to %q %q in %q",
		lc.ewoqWallet.xChainAddr,
		to.name,
		to.xChainAddr,
		nodeName,
	)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	amount := 100000 * units.Avax
	txID, err := cli.XChainWalletAPI().Send(
		userPass,
		[]string{lc.ewoqWallet.xChainAddr}, // from
		"",                                 // changeAddr
		amount,                             // amount
		"AVAX",                             // asset
		to.xChainAddr,                      // to
		"hi!",                              // message
	)
	if err != nil {
		return err
	}
	return lc.checkXChainTx(nodeName, txID)
}

func (lc *localNetwork) exportX(nodeName string, from *wallet) error {
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	utxos, err := lc.getUTXOs(
		func() ([][]byte, error) {
			ubs, _, err := cli.XChainAPI().GetUTXOs(
				[]string{from.xChainAddr},
				100,
				"",
				"",
			)
			return ubs, err
		},
		xCodecManager,
	)
	if err != nil {
		return err
	}

	color.Blue("exporting X-Chain asset %q from %q to %q",
		lc.avaxAssetID,
		from.xChainAddr,
		from.pChainAddr,
	)
	amount := 50000 * units.Avax
	totalOut, ins, signers := lc.getInputs(utxos, from, amount)

	outs := []*avax.TransferableOutput{}
	if totalOut-amount-units.MilliAvax > 0 {
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: lc.avaxAssetID},
			Out: &secp256k1fx.TransferOutput{
				Amt: totalOut - amount - units.MilliAvax, // deduct X-Chain gas fee .001 AVAX
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{from.shortAddr},
				},
			},
		})
	}
	xTx := avm.Tx{UnsignedTx: &avm.ExportTx{
		BaseTx: avm.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    1337, // same as network-runner genesis
			BlockchainID: lc.xChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		DestinationChain: lc.pChainID,
		ExportedOuts: []*avax.TransferableOutput{{
			Asset: avax.Asset{ID: lc.avaxAssetID},
			Out: &secp256k1fx.TransferOutput{
				Amt: amount,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{from.shortAddr},
				},
			},
		}},
	}}
	if err := xTx.SignSECP256K1Fx(xCodecManager, signers); err != nil {
		return fmt.Errorf("unable to sign X-Chain export tx: %w", err)
	}
	txID, err := cli.XChainAPI().IssueTx(xTx.Bytes())
	if err != nil {
		return fmt.Errorf("failed to issue tx: %w", err)
	}
	return lc.checkXChainTx(nodeName, txID)
}

func (lc *localNetwork) importP(nodeName string, from *wallet) error {
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	utxos, err := lc.getUTXOs(
		func() ([][]byte, error) {
			ubs, _, err := cli.PChainAPI().GetAtomicUTXOs(
				[]string{from.pChainAddr},
				lc.xChainID.String(),
				100,
				"",
				"",
			)
			return ubs, err
		},
		pCodecManager,
	)
	if err != nil {
		return err
	}

	color.Blue("importing P-Chain asset %q from %q to %q",
		lc.avaxAssetID,
		from.xChainAddr,
		from.pChainAddr,
	)
	totalOut, ins, signers := lc.getInputs(utxos, from, 0)
	pTx := platformvm.Tx{UnsignedTx: &platformvm.UnsignedImportTx{
		BaseTx: platformvm.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    1337, // same as network-runner genesis
			BlockchainID: lc.pChainID,
			Outs: []*avax.TransferableOutput{{
				Asset: avax.Asset{ID: lc.avaxAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: totalOut - units.MilliAvax, // deduct X-Chain gas fee .001 AVAX
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{from.shortAddr},
					},
				},
			}},
		}},
		SourceChain:    lc.xChainID,
		ImportedInputs: ins,
	}}
	if err := pTx.Sign(pCodecManager, signers); err != nil {
		return fmt.Errorf("unable to sign P-Chain import tx: %w", err)
	}
	txID, err := cli.PChainAPI().IssueTx(pTx.Bytes())
	if err != nil {
		return fmt.Errorf("failed to issue tx: %w", err)
	}
	return lc.checkPChainTx(nodeName, txID)
}

func (lc *localNetwork) withdrawEwoqCChainToWallet(nodeName string, to *wallet) error {
	color.Blue("withdrawing C-Chain funds from ewoq %q to wallet %q in %q",
		lc.ewoqWallet.cChainAddr,
		to.cChainAddr,
		nodeName,
	)
	cli, ok := lc.apiClis[nodeName]
	if !ok {
		return fmt.Errorf("%q API client not found", nodeName)
	}
	_ = cli

	return nil
}

func (lc *localNetwork) getUTXOs(get func() ([][]byte, error), cd codec.Manager) ([]*avax.UTXO, error) {
	ubs, err := get()
	if err != nil {
		return nil, err
	}
	utxos := make([]*avax.UTXO, 0)
	for _, ub := range ubs {
		utxo := new(avax.UTXO)
		if _, err := cd.Unmarshal(ub, utxo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal utxo bytes: %w", err)
		}
		if utxo.AssetID() != lc.avaxAssetID {
			continue
		}
		utxos = append(utxos, utxo)
	}
	return utxos, err
}

// get inputs to spend from the wallet
func (lc *localNetwork) getInputs(
	utxos []*avax.UTXO,
	w *wallet,
	targetAmount uint64) (
	uint64,
	[]*avax.TransferableInput,
	[][]*crypto.PrivateKeySECP256K1R,
) {
	totalOut := uint64(0)
	ins := make([]*avax.TransferableInput, 0)
	signers := make([][]*crypto.PrivateKeySECP256K1R, 0)
	for _, utxo := range utxos {
		inputf, inputSigners, err := w.keyChain.Spend(utxo.Out, 0)
		if err != nil {
			color.Cyan("this utxo cannot be spend with current keys %v; skipping", err)
			continue
		}
		input, ok := inputf.(avax.TransferableIn)
		if !ok {
			color.Cyan("this utxo unexpected type %T; skipping", inputf)
			continue
		}
		totalOut += input.Amount()
		ins = append(ins, &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: lc.avaxAssetID},
			In:     input,
		})
		signers = append(signers, inputSigners)
		if targetAmount > 0 && totalOut > targetAmount+units.MilliAvax {
			break
		}
	}
	avax.SortTransferableInputsWithSigners(ins, signers)
	return totalOut, ins, signers
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

		// TODO: fetch by JSON
		// txBytes, err := xcli.GetTx(txID)

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
