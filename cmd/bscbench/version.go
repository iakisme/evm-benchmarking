package main

import (
	"runtime/debug"

	// Import a lightweight BSC package so the module appears in debug.BuildInfo.Deps.
	_ "github.com/ethereum/go-ethereum/params"
	"github.com/spf13/cobra"
)

// Version is set via -ldflags at build time.
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print bscbench and BSC dependency versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("bscbench=%s bsc=%s go=%s\n",
				Version, bscDepVersion(), runtimeGoVersion())
			return nil
		},
	}
}

func bscDepVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/ethereum/go-ethereum" {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "none"
}

func runtimeGoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return info.GoVersion
}
