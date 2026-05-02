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
			info, _ := debug.ReadBuildInfo()
			cmd.Printf("bscbench=%s bsc=%s go=%s\n",
				Version, bscDepVersion(info), goVersion(info))
			return nil
		},
	}
}

func bscDepVersion(info *debug.BuildInfo) string {
	if info == nil {
		return "unknown"
	}
	for _, dep := range info.Deps {
		// BSC is a fork of go-ethereum that retains the upstream module path.
		// When pinned via a replace directive, dep.Replace carries the BSC tag.
		if dep.Path == "github.com/ethereum/go-ethereum" {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "none"
}

func goVersion(info *debug.BuildInfo) string {
	if info == nil {
		return "unknown"
	}
	return info.GoVersion
}
