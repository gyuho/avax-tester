// avax-tester is a set of Avalanche test commands.
package main

import (
	"fmt"
	"os"

	"github.com/gyuho/avax-tester/cmd/avax-tester/local"
	"github.com/gyuho/avax-tester/cmd/avax-tester/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "avax-tester",
	Short:      "Avalanche test CLI",
	SuggestFor: []string{"avaxtest"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		local.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "avax-tester failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
