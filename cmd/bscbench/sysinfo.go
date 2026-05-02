package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kai-w/bscbench/internal/sysinfo"
)

func newSysinfoCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "sysinfo",
		Short: "Collect host inventory and write JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			si, err := sysinfo.Collect(context.Background(), []string{"/"})
			if err != nil {
				return fmt.Errorf("collect: %w", err)
			}
			b, err := json.MarshalIndent(si, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}
			b = append(b, '\n')
			if out == "" || out == "-" {
				_, err = os.Stdout.Write(b)
				return err
			}
			return os.WriteFile(out, b, 0o644)
		},
	}
	cmd.Flags().StringVar(&out, "out", "-", "Output file (default: stdout)")
	return cmd
}
