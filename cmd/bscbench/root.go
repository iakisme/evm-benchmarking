package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "bscbench",
		Short:         "BSC EVM benchmark tool",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newVersionCmd(), newSysinfoCmd())
	return cmd
}
