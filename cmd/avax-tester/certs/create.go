package certs

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/perms"
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

	firstNodeFullID := ""
	for i := 0; i < nodes; i++ {
		keyPath := filepath.Join(dirPath, fmt.Sprintf("staker%d.key", i+1))
		certPath := filepath.Join(dirPath, fmt.Sprintf("staker%d.crt", i+1))

		cert, certBytes, err := writeNodeStakingKeyPair(keyPath, certPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create %q and %q (%v)\n", keyPath, certPath, err)
			os.Exit(1)
		}

		fmt.Printf("----------\n")
		fmt.Printf(colorize(logColor, `[light_green][%02d] certificate[default]
--staking-tls-key-file=%s \
--staking-tls-cert-file=%s`), i+1, keyPath, certPath)
		if firstNodeFullID != "" {
			fmt.Printf(` \
--bootstrap-ids=%s`, firstNodeFullID)
		}
		fmt.Printf("\n\n")

		if i == 0 {
			// NOTE: hashing.PubkeyBytesToAddress(certBytes) does not work...
			_ = certBytes
			// ref. node/Node.Initialize
			id, err := ids.ToShortID(hashing.PubkeyBytesToAddress(cert.Leaf.Raw))
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create node ID %v\n", err)
			}
			firstNodeFullID = id.PrefixedString(constants.NodeIDPrefix)
		}
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'avax-tester certs create --dir-path %q' success\n", dirPath)
}

func writeNodeStakingKeyPair(keyPath, certPath string) (
	cert *tls.Certificate,
	certBytes []byte,
	err error) {
	certBytes, keyBytes, err := staking.NewCertAndKeyBytes()
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(filepath.Dir(certPath), perms.ReadWriteExecute); err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), perms.ReadWriteExecute); err != nil {
		return nil, nil, err
	}

	certFile, err := os.Create(certPath)
	if err != nil {
		return nil, nil, err
	}
	if _, err = certFile.Write(certBytes); err != nil {
		return nil, nil, err
	}
	if err = certFile.Close(); err != nil {
		return nil, nil, err
	}
	if err = os.Chmod(certPath, perms.ReadOnly); err != nil {
		return nil, nil, err
	}

	// Write key to disk
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return nil, nil, err
	}
	if _, err := keyOut.Write(keyBytes); err != nil {
		return nil, nil, err
	}
	if err := keyOut.Close(); err != nil {
		return nil, nil, err
	}
	if err := os.Chmod(keyPath, perms.ReadOnly); err != nil {
		return nil, nil, err
	}

	cert, err = staking.LoadTLSCert(keyPath, certPath)
	if err != nil {
		return nil, nil, err
	}
	return cert, certBytes, nil
}
