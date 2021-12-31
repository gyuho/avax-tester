// Package run implements runner command.
package run

import (
	"github.com/gyuho/avax-tester/runner"
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	avalancheGoBinPath string
	vmName             string
	vmID               string
	vmGenesisPath      string
	outputPath         string
)

const defaultVMID = "tGas3T58KzdjLHhBDMnH2TvrddhqTji5iZAMZ3RXs2NLpSnhH"

// NewCommand implements "avax-tester version" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runner",
		Short: "Runs avalanche cluster",
		RunE:  runFunc,
	}
	cmd.PersistentFlags().StringVar(
		&avalancheGoBinPath,
		"avalanchego-path",
		"",
		"avalanchego binary path",
	)
	cmd.PersistentFlags().StringVar(
		&vmName,
		"vm-name",
		"",
		"VM name",
	)
	cmd.PersistentFlags().StringVar(
		&vmID,
		"vm-id",
		defaultVMID,
		"VM ID (must be formatted ids.ID)",
	)
	cmd.PersistentFlags().StringVar(
		&vmGenesisPath,
		"vm-genesis-path",
		"",
		"VM genesis file path",
	)
	cmd.PersistentFlags().StringVar(
		&outputPath,
		"output-path",
		"",
		"output YAML path to write local cluster information",
	)
	return cmd
}

func runFunc(cmd *cobra.Command, args []string) error {
	return runner.Run(avalancheGoBinPath, vmName, vmID, vmGenesisPath, outputPath)
}
