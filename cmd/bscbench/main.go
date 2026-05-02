package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bscbench:", err)
		os.Exit(1)
	}
}

func run() error {
	return newRootCmd().Execute()
}
