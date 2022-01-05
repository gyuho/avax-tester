package runner

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type wallet struct {
	name           string
	privKey        *crypto.PrivateKeySECP256K1R
	privKeyEncoded string
	keyChain       *secp256k1fx.Keychain
	commonAddr     common.Address
	shortAddr      ids.ShortID
	xChainAddr     string
	xChainBal      uint64
	pChainAddr     string
	pChainBal      uint64
	cChainAddr     string
	cChainBal      uint64
}

func newWallet(name string, privKey *crypto.PrivateKeySECP256K1R) *wallet {
	rawPrivKey := privKey.Bytes()
	enc, err := formatting.EncodeWithChecksum(formatting.CB58, rawPrivKey)
	if err != nil {
		panic(err)
	}
	privKeyEncoded := "PrivateKey-" + enc

	keyChain := secp256k1fx.NewKeychain()
	keyChain.Add(privKey)

	commonAddr := ethcrypto.PubkeyToAddress(privKey.ToECDSA().PublicKey)
	shortAddr := privKey.PublicKey().Address()
	fmt.Println("common", commonAddr, shortAddr)

	xAddr, err := formatting.FormatAddress("X", "custom", privKey.PublicKey().Address().Bytes())
	if err != nil {
		panic(err)
	}
	pAddr, err := formatting.FormatAddress("P", "custom", privKey.PublicKey().Address().Bytes())
	if err != nil {
		panic(err)
	}
	cAddr, err := formatting.FormatAddress("C", "custom", privKey.PublicKey().Address().Bytes())
	if err != nil {
		panic(err)
	}

	return &wallet{
		name:           name,
		privKey:        privKey,
		privKeyEncoded: privKeyEncoded,
		keyChain:       keyChain,
		commonAddr:     commonAddr,
		shortAddr:      shortAddr,
		xChainAddr:     xAddr,
		xChainBal:      0,
		pChainAddr:     pAddr,
		pChainBal:      0,
		cChainAddr:     cAddr,
		cChainBal:      0,
	}
}
