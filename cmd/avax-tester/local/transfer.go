package local

import (
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/api/keystore"
	"github.com/gyuho/avax-tester/pkg/randutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/ybbus/jsonrpc/v2"
)

func newTransfer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Initiates a transfer transaction",
		Run:   transferFunc,
	}
	return cmd
}

const (
	// this is a private key used for testing only
	// ref. https://docs.avax.network/build/tutorials/platform/create-a-local-test-network
	preFundedKey   = "PrivateKey-ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	requestTimeout = 5 * time.Second
)

var (
	host1 = "http://localhost:9650"
	user1 = api.UserPass{
		Username: randutil.String(10),
		Password: randutil.String(10),
	}
)

func transferFunc(cmd *cobra.Command, args []string) {
	fmt.Printf("\n*********************************\n\n")
	if enablePrompt {
		prompt := promptui.Select{
			Label: fmt.Sprintf("Ready to transfer funds from %q, should we continue?", host1),
			Items: []string{
				"No, cancel it!",
				"Yes, let's transfer!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'transfer' [index %d, answer %q]\n", idx, answer)
			return
		}
	}

	// step 1. create a user in the local keystore
	keyStoreClient := keystore.NewClient(host1, requestTimeout)
	if success, err := keyStoreClient.CreateUser(user1); !success || err != nil {
		fmt.Fprintln(os.Stderr, "failed to create a user", success, err)
		os.Exit(1)
	}

	xChainEp1 := fmt.Sprintf("%s/ext/bc/X", host1)
	pChainEp1 := fmt.Sprintf("%s/ext/bc/P", host1)
	cChainEp1 := fmt.Sprintf("%s/ext/bc/C/avax", host1)

	// step 2. import the pre-funded private key to the chains and create addresses
	var xChainImportKeyResp importKeyResponse
	if err := jsonrpc.NewClient(xChainEp1).CallFor(
		&xChainImportKeyResp,
		"avm.importKey",
		struct {
			username   string
			password   string
			privateKey string
		}{
			user1.Username,
			user1.Password,
			preFundedKey,
		}); err != nil {
		fmt.Fprintln(os.Stderr, "failed avm.importKey", err)
		os.Exit(1)
	}
	var pChainImportKeyResp importKeyResponse
	if err := jsonrpc.NewClient(pChainEp1).CallFor(
		&pChainImportKeyResp,
		"platform.importKey",
		struct {
			username   string
			password   string
			privateKey string
		}{
			user1.Username,
			user1.Password,
			preFundedKey,
		}); err != nil {
		fmt.Fprintln(os.Stderr, "failed platform.importKey", err)
		os.Exit(1)
	}
	var cChainImportKeyResp importKeyResponse
	if err := jsonrpc.NewClient(cChainEp1).CallFor(
		&cChainImportKeyResp,
		"avax.importKey",
		struct {
			username   string
			password   string
			privateKey string
		}{
			user1.Username,
			user1.Password,
			preFundedKey,
		}); err != nil {
		fmt.Fprintln(os.Stderr, "failed avax.importKey", err)
		os.Exit(1)
	}

	// step 3. get the list of addresses for the pre-funded key
	var xChainListAddressResp listAddressResponse
	if err := jsonrpc.NewClient(xChainEp1).CallFor(
		&xChainListAddressResp,
		"avm.listAddresses",
		struct {
			username string
			password string
		}{
			user1.Username,
			user1.Password,
		}); err != nil {
		fmt.Fprintln(os.Stderr, "failed avm.listAddresses", err)
		os.Exit(1)
	}
	var pChainListAddressResp listAddressResponse
	if err := jsonrpc.NewClient(xChainEp1).CallFor(
		&pChainListAddressResp,
		"platform.listAddresses",
		struct {
			username string
			password string
		}{
			user1.Username,
			user1.Password,
		}); err != nil {
		fmt.Fprintln(os.Stderr, "failed platform.listAddresses", err)
		os.Exit(1)
	}
	var cChainListAddressResp importKeyResponse
	if err := jsonrpc.NewClient(xChainEp1).CallFor(
		&cChainListAddressResp,
		"avax.importKey",
		struct {
			username   string
			password   string
			privateKey string
		}{
			user1.Username,
			user1.Password,
			preFundedKey,
		}); err != nil {
		fmt.Fprintln(os.Stderr, "failed avax.importKey", err)
		os.Exit(1)
	}
	if xChainImportKeyResp.Result.Address != xChainListAddressResp.Result.Addresses[0] {
		fmt.Fprintf(
			os.Stderr,
			"unexpected xChainListAddressResp.Result.Addresses %q, expected %q",
			xChainListAddressResp.Result.Addresses,
			xChainImportKeyResp.Result.Address,
		)
		os.Exit(1)
	}
	if pChainImportKeyResp.Result.Address != pChainListAddressResp.Result.Addresses[0] {
		fmt.Fprintf(
			os.Stderr,
			"unexpected pChainListAddressResp.Result.Addresses %q, expected %q",
			pChainListAddressResp.Result.Addresses,
			pChainImportKeyResp.Result.Address,
		)
		os.Exit(1)
	}
	fmt.Printf(colorize(logColor, "[light_green]X-chain address [default]%q\n"), xChainImportKeyResp.Result.Address)
	fmt.Printf(colorize(logColor, "[light_green]P-chain address [default]%q\n"), pChainImportKeyResp.Result.Address)
	fmt.Printf(colorize(logColor, "[light_green]C-chain address [default]%q\n"), cChainImportKeyResp.Result.Address)

	// step 4. get the balance of the pre-funded wallet

	// step 5. create another address in the X-chain for transfer
	//         TODO: create in another node?

	// step 6. check the balance and transfer from one to another

	// step 7. check the status of the transaction
}

type importKeyResponse struct {
	Result importKeyResult `json:"result"`
}

type importKeyResult struct {
	Address string `json:"address"`
}

type listAddressResponse struct {
	Result listAddressResult `json:"result"`
}

type listAddressResult struct {
	Addresses []string `json:"addresses"`
}
