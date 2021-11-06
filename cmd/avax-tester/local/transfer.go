package local

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/api/keystore"
	"github.com/gyuho/avax-tester/pkg/randutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/ybbus/jsonrpc/v2"
)

const (
	// this is a private key used for testing only
	// ref. https://docs.avax.network/build/tutorials/platform/create-a-local-test-network
	preFundedKey   = "PrivateKey-ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	requestTimeout = 5 * time.Second
)

var (
	apiHosts []string
)

func newTransfer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Initiates a transfer transaction",
		Run:   transferFunc,
	}
	cmd.PersistentFlags().StringSliceVar(&apiHosts, "api-hosts", []string{"http://127.0.0.1:9650"}, "Hosts for API endpoints")
	return cmd
}

func transferFunc(cmd *cobra.Command, args []string) {
	if len(apiHosts) == 0 {
		fmt.Fprintln(os.Stderr, "'--api-hosts' flag is empty")
		panic(1)
	}

	fmt.Printf("\n*********************************\n\n")
	if enablePrompt {
		prompt := promptui.Select{
			Label: fmt.Sprintf("Ready to transfer funds with hosts %q, should we continue?", apiHosts),
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

	users := make([]api.UserPass, len(apiHosts))
	for i := range users {
		users[i] = api.UserPass{
			Username: randutil.String(10),
			Password: randutil.String(10) + "!@##@$!#$!@#!@#",
		}
	}

	fmt.Println(colorize(logColor, `
[yellow]-----
step 1. create a user in the local keystore
[default]`))
	for i, host := range apiHosts {
		if success, err := keystore.NewClient(host, requestTimeout).CreateUser(users[i]); !success || err != nil {
			fmt.Fprintln(os.Stderr, "failed to create a user", success, err)
			panic(1)
		}
		fmt.Printf(colorize(logColor, "[light_green]Created user %q [default]%+v\n"), host, users[i])
	}

	xChainEp1 := fmt.Sprintf("%s/ext/bc/X", apiHosts[0])
	pChainEp1 := fmt.Sprintf("%s/ext/bc/P", apiHosts[0])
	cChainEp1 := fmt.Sprintf("%s/ext/bc/C/avax", apiHosts[0])

	fmt.Println(colorize(logColor, `
[yellow]-----
step 2. import the pre-funded private key to the chains and create addresses
[default]`))
	rr, err := jsonrpc.NewClient(xChainEp1).Call("avm.importKey", struct {
		UserName   string `json:"username"`
		Password   string `json:"password"`
		PrivateKey string `json:"privateKey"`
	}{
		users[0].Username,
		users[0].Password,
		preFundedKey,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed avm.importKey", err)
		panic(1)
	}
	rm, ok := rr.Result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected rr.Result", reflect.TypeOf(rr.Result))
		panic(1)
	}
	xChainAddress := fmt.Sprint(rm["address"])

	rr, err = jsonrpc.NewClient(pChainEp1).Call("platform.importKey", struct {
		UserName   string `json:"username"`
		Password   string `json:"password"`
		PrivateKey string `json:"privateKey"`
	}{
		users[0].Username,
		users[0].Password,
		preFundedKey,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed platform.importKey", err)
		panic(1)
	}
	rm, ok = rr.Result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected rr.Result", reflect.TypeOf(rr.Result))
		panic(1)
	}
	pChainAddress := fmt.Sprint(rm["address"])

	rr, err = jsonrpc.NewClient(cChainEp1).Call("avax.importKey", struct {
		UserName   string `json:"username"`
		Password   string `json:"password"`
		PrivateKey string `json:"privateKey"`
	}{
		users[0].Username,
		users[0].Password,
		preFundedKey,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed avax.importKey", err)
		panic(1)
	}
	rm, ok = rr.Result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected rr.Result", reflect.TypeOf(rr.Result))
		panic(1)
	}
	cChainAddress := fmt.Sprint(rm["address"])

	fmt.Println(colorize(logColor, `
[yellow]-----
step 3. get the list of addresses for the pre-funded key
[default]`))
	rr, err = jsonrpc.NewClient(xChainEp1).Call("avm.listAddresses", struct {
		UserName string `json:"username"`
		Password string `json:"password"`
	}{
		users[0].Username,
		users[0].Password,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed avm.listAddresses", err)
		panic(1)
	}
	rm, ok = rr.Result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected rr.Result", reflect.TypeOf(rr.Result))
		panic(1)
	}
	xChainAddresses, _ := rm["addresses"].([]interface{})
	if xChainAddress != fmt.Sprint(xChainAddresses[0]) {
		fmt.Fprintf(os.Stderr, "unexpected xChainAddress %v, expected %q\n", xChainAddress[0], xChainAddress)
		panic(1)
	}

	rr, err = jsonrpc.NewClient(pChainEp1).Call("platform.listAddresses", struct {
		UserName string `json:"username"`
		Password string `json:"password"`
	}{
		users[0].Username,
		users[0].Password,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed platform.listAddresses", err)
		panic(1)
	}
	rm, ok = rr.Result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected rr.Result", reflect.TypeOf(rr.Result))
		panic(1)
	}
	pChainAddresses, _ := rm["addresses"].([]interface{})
	if pChainAddress != fmt.Sprint(pChainAddresses[0]) {
		fmt.Fprintf(os.Stderr, "unexpected pChainAddress %v, expected %q\n", pChainAddress[0], xChainAddress)
		panic(1)
	}
	fmt.Printf(colorize(logColor, "[light_green]X-chain address [default]%q\n"), xChainAddress)
	fmt.Printf(colorize(logColor, "[light_green]P-chain address [default]%q\n"), pChainAddress)
	fmt.Printf(colorize(logColor, "[light_green]C-chain address [default]%q\n"), cChainAddress)

	fmt.Println(colorize(logColor, `
[yellow]-----
step 4. get the balance of the pre-funded wallet
[default]`))
	rr, err = jsonrpc.NewClient(xChainEp1).Call("avm.getBalance", struct {
		Address string `json:"address"`
		AssetID string `json:"assetID"`
	}{
		xChainAddress,
		"AVAX",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed avm.getBalance", err)
		panic(1)
	}
	rm, ok = rr.Result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected rr.Result", reflect.TypeOf(rr.Result))
		panic(1)
	}
	xChainBalance := rm["balance"]
	if xChainBalance != "300000000000000000" {
		fmt.Fprintf(
			os.Stderr,
			"unexpected xChainBalance %q, expected 300000000000000000",
			xChainBalance,
		)
		panic(1)
	}

	rr, err = jsonrpc.NewClient(pChainEp1).Call("platform.getBalance", struct {
		Address string `json:"address"`
	}{
		pChainAddress,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed platform.getBalance", err)
		panic(1)
	}
	rm, ok = rr.Result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected rr.Result", reflect.TypeOf(rr.Result))
		panic(1)
	}
	pChainBalance := rm["balance"]
	if pChainBalance != "30000000000000000" {
		fmt.Fprintf(
			os.Stderr,
			"unexpected pChainBalance %q, expected 30000000000000000",
			pChainBalance,
		)
		panic(1)
	}
	fmt.Printf(colorize(logColor, "[light_green]X-chain balance [default]%q\n"), xChainBalance)
	fmt.Printf(colorize(logColor, "[light_green]P-chain balance [default]%q\n"), pChainBalance)

	// TODO: create in another node?
	fmt.Println(colorize(logColor, `
[yellow]-----
step 5. create another address in the X-chain for transfer
[default]`))
	// TODO

	fmt.Println(colorize(logColor, `
[yellow]-----
step 6. check the balance and transfer from one to another
[default]`))
	// TODO

	fmt.Println(colorize(logColor, `
[yellow]-----
step 7. check the status of the transaction
[default]`))
	// TODO
}
