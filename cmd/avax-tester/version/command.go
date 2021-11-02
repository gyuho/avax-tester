// Package version implements version command.
package version

import (
	"fmt"

	"github.com/gyuho/avax-tester/version"
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "avax-tester version" command.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints out avax-tester version",
		Run:   versionFunc,
	}
}

func versionFunc(cmd *cobra.Command, args []string) {
	fmt.Println(version.Version())
}
