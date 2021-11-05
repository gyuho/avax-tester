package local

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	nodes         int
	dbDirPath     string
	certsDirPath  string
	cmdOutputPath string
)

func newCreate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Writes certificates and commands",
		Run:   createFunc,
	}
	cmd.PersistentFlags().IntVar(&nodes, "nodes", 5, "number of nodes to generate certs")
	cmd.PersistentFlags().StringVar(&dbDirPath, "db-dir-path", filepath.Join(os.TempDir(), "avax-tester-db"), "directory to output database files")
	cmd.PersistentFlags().StringVar(&certsDirPath, "certs-dir-path", filepath.Join(os.TempDir(), "avax-tester-certs"), "directory to output generated certs, leave empty to disable staking")
	cmd.PersistentFlags().StringVar(&cmdOutputPath, "cmd-output-path", filepath.Join(os.TempDir(), "avax-tester-cmd.sh"), "file path to write commands")
	return cmd
}

func createFunc(cmd *cobra.Command, args []string) {
	if nodes == 0 {
		fmt.Fprintln(os.Stderr, "'--nodes' flag is set to 0")
		os.Exit(1)
	}
	if dbDirPath == "" {
		fmt.Fprintln(os.Stderr, "'--db-dir-path' flag is not specified")
		os.Exit(1)
	}
	if certsDirPath == "" {
		fmt.Fprintln(os.Stderr, "'--certs-dir-path' flag is not specified")
		os.Exit(1)
	}
	if cmdOutputPath == "" {
		fmt.Fprintln(os.Stderr, "'--cmd-output-path' flag is not specified")
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n\n")
	if enablePrompt {
		prompt := promptui.Select{
			Label: fmt.Sprintf("Ready to create/overwrite database %q and certs %q, should we continue?", dbDirPath, certsDirPath),
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

	fmt.Printf("\n\ncleaning up %q and %q for %d nodes\n\n", dbDirPath, certsDirPath, nodes)
	if err := os.RemoveAll(dbDirPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete %q (%v)\n", dbDirPath, err)
		os.Exit(1)
	}
	if err := os.RemoveAll(certsDirPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete %q (%v)\n", certsDirPath, err)
		os.Exit(1)
	}
	if err := os.RemoveAll(cmdOutputPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete %q (%v)\n", cmdOutputPath, err)
		os.Exit(1)
	}

	all := make([]avalancheGo, nodes)
	for i := 0; i < nodes; i++ {
		nodeName := fmt.Sprintf("s%d", i+1)
		keyPath := filepath.Join(certsDirPath, fmt.Sprintf("%s.key", nodeName))
		certPath := filepath.Join(certsDirPath, fmt.Sprintf("%s.crt", nodeName))
		cert, _, err := writeNodeStakingKeyPair(keyPath, certPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create %q and %q (%v)\n", keyPath, certPath, err)
			os.Exit(1)
		}
		// NOTE: hashing.PubkeyBytesToAddress(certBytes) does not work...
		// ref. avalanchego/node/Node.Initialize
		id, err := ids.ToShortID(hashing.PubkeyBytesToAddress(cert.Leaf.Raw))
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create node ID %v", err)
			os.Exit(1)
		}
		nodeID := id.PrefixedString(constants.NodeIDPrefix)

		all[i] = avalancheGo{
			NodeName:           nodeName,
			NodeID:             nodeID,
			Binary:             "./build/avalanchego",
			LogLevel:           "info",
			NetworkID:          "local",
			PublicIP:           "127.0.0.1",
			HTTPPort:           9650 + i*2,
			SnowSampleSize:     nodes / 2,
			SnowQuorumSize:     nodes / 2,
			DBDir:              filepath.Join(dbDirPath, nodeName),
			StakingEnabled:     true,
			StakingPort:        9650 + i*2 + 1,
			BootstrapIPs:       "", // first beacon node sets this empty
			BootstrapIDs:       "", // first beacon node sets this empty
			StakingTLSKeyFile:  keyPath,
			StakingTLSCertFile: certPath,
		}
		if i > 0 { // only populate non-beacon nodes
			all[i].BootstrapIPs = fmt.Sprintf("127.0.0.1:%d", all[0].StakingPort)
			all[i].BootstrapIDs = all[0].NodeID
		}

		fmt.Printf(colorize(logColor, "[yellow]created %q for %q\n"), certPath, nodeID)
	}

	println()
	ss := tmplAvalancheGoBash
	for i, av := range all {
		s := av.String()
		fmt.Printf(colorize(logColor, `
[light_green]-----
# [%02d][default]
%s
`), i+1, s)
		ss += fmt.Sprintf("# [%02d]\n%s\n\n", i+1, s)
	}
	fmt.Println(curlCmd)
	if err := ioutil.WriteFile(cmdOutputPath, []byte(ss+curlCmd), 0777); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %q (%v)\n", cmdOutputPath, err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'avax-tester local create' success!\n\ncat %q\n", cmdOutputPath)
}

const curlCmd = `
# use this to test API
curl -X POST --data '{
	"jsonrpc":"2.0",
	"id"     :1,
	"method" :"info.peers"
}' \
-H 'content-type:application/json;' \
127.0.0.1:9650/ext/info

`

type avalancheGo struct {
	NodeName           string
	NodeID             string
	Binary             string
	LogLevel           string
	NetworkID          string
	PublicIP           string
	HTTPPort           int
	SnowSampleSize     int
	SnowQuorumSize     int
	DBDir              string
	StakingEnabled     bool
	StakingPort        int
	BootstrapIPs       string
	BootstrapIDs       string
	StakingTLSKeyFile  string
	StakingTLSCertFile string
}

const tmplAvalancheGoCmd = `# commands for {{.NodeName}}, {{.NodeID}}
kill -9 $(lsof -t -i:{{.HTTPPort}})
kill -9 $(lsof -t -i:{{.StakingPort}}){{if .StakingEnabled}}
openssl x509 -in {{.StakingTLSCertFile}} -text -noout{{end}}
cd ${HOME}/go/src/github.com/ava-labs/avalanchego
{{.Binary}} \
--log-level={{.LogLevel}} \
--network-id={{.NetworkID}} \
--public-ip={{.PublicIP}} \
--http-port={{.HTTPPort}} \
--snow-sample-size={{.SnowSampleSize}} \
--snow-quorum-size={{.SnowQuorumSize}} \
--db-dir={{.DBDir}} \
--staking-enabled={{.StakingEnabled}} \
--staking-port={{.StakingPort}} \
--bootstrap-ips={{.BootstrapIPs}} \
--bootstrap-ids={{.BootstrapIDs}}{{if .StakingEnabled}} \
--staking-tls-key-file={{.StakingTLSKeyFile}} \
--staking-tls-cert-file={{.StakingTLSCertFile}}{{end}}
`

const tmplAvalancheGoBash = `#!/bin/bash
set -e
set -x


`

func (ag avalancheGo) String() string {
	t := template.Must(template.New("tmplAvalancheGoCmd").Parse(tmplAvalancheGoCmd))
	var buf bytes.Buffer
	if err := t.Execute(&buf, ag); err != nil {
		panic(err)
	}
	return buf.String()
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
