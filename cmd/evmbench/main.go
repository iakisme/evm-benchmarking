package main

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/log"
)

func main() {
	// BSC's default logger uses a DiscardHandler — so any internal
	// log.Crit/Error/Warn fired from inside go-ethereum/BSC code (e.g.
	// pathdb journal failures, rawdb open errors) would otherwise vanish
	// silently while still triggering os.Exit. Install a stderr handler
	// at warn level so those messages surface.
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(
		os.Stderr, log.LevelWarn, false /*useColor*/)))

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "evmbench:", err)
		os.Exit(1)
	}
}

func run() error {
	return newRootCmd().Execute()
}
