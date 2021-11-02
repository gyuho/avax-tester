// Package certs implements certificates related commands.
package certs

import (
	"github.com/mitchellh/colorstring"
	"github.com/spf13/cobra"
)

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	enablePrompt bool
	logColor     bool
)

// NewCommand implements "aws-k8s-tester eks" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "certs",
		Short: "certs commands",
	}
	cmd.PersistentFlags().BoolVarP(&enablePrompt, "enable-prompt", "e", true, "'true' to enable prompt mode")
	cmd.PersistentFlags().BoolVarP(&logColor, "log-color", "c", true, "'true' to enable log color")
	cmd.AddCommand(
		newCreate(),
	)
	return cmd
}

func colorize(logColor bool, input string) string {
	colorize := colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: !logColor,
		Reset:   true,
	}
	return colorize.Color(input)
}
