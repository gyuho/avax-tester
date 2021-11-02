package certs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanchego/staking"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	dirPath string
	nodes   int
)

func newCreate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Writes certificates",
		Run:   createFunc,
	}
	cmd.PersistentFlags().StringVarP(&dirPath, "dir-path", "d", filepath.Join(os.TempDir(), "avax-tester-certs"), "directory to output generates certs")
	cmd.PersistentFlags().IntVarP(&nodes, "nodes", "n", 5, "number of nodes to generate certs")
	return cmd
}

func createFunc(cmd *cobra.Command, args []string) {
	if dirPath == "" {
		fmt.Fprintln(os.Stderr, "'--dir-path' flag is not specified")
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	if enablePrompt {
		prompt := promptui.Select{
			Label: fmt.Sprintf("Ready to create/overwrite certs to %q, should we continue?", dirPath),
			Items: []string{
				"No, cancel it!",
				"Yes, let's create!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'create' [index %d, answer %q]\n", idx, answer)
			return
		}
	}
	fmt.Printf("\noverwriting %d cert files\n\n", nodes)
	if err := os.RemoveAll(dirPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete %q (%v)\n", dirPath, err)
		os.Exit(1)
	}
	for i := 0; i < nodes; i++ {
		keyPath := filepath.Join(dirPath, fmt.Sprintf("s%d-key.pem", i+1))
		certPath := filepath.Join(dirPath, fmt.Sprintf("s%d.pem", i+1))
		if err := staking.InitNodeStakingKeyPair(keyPath, certPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create %q and %q (%v)\n", keyPath, certPath, err)
			os.Exit(1)
		}
		fmt.Printf("----------\n")
		fmt.Printf(colorize(logColor, "[light_green][%02d] certificate\n[default]--staking-tls-key-file=%s\n--staking-tls-cert-file=%s\n\n"), i+1, keyPath, certPath)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'avax-tester certs create --dir-path %q' success\n", dirPath)
}
