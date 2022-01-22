// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"os"

	"github.com/gyuho/avax-tester/cmd/avalanche-network-runner/control"
	"github.com/gyuho/avax-tester/cmd/avalanche-network-runner/ping"
	"github.com/gyuho/avax-tester/cmd/avalanche-network-runner/server"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "avalanche-network-runner",
	Short:      "avalanche-network-runner commands",
	SuggestFor: []string{"network-runner"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		server.NewCommand(),
		ping.NewCommand(),
		control.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "avalanche-network-runner failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
