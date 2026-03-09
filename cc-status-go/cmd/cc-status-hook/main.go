package main

import (
	"fmt"
	"io"
	"os"

	"github.com/anthropics/cc-status-go/internal/hook"
)

func main() {
	// Handle install/uninstall subcommands
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "install":
			if err := hook.Install(); err != nil {
				fmt.Fprintf(os.Stderr, "Error installing hooks: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "uninstall":
			if err := hook.Uninstall(); err != nil {
				fmt.Fprintf(os.Stderr, "Error uninstalling hooks: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// Default: read stdin, parse, and send to socket
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		// Always exit 0 to never block Claude Code
		os.Exit(0)
	}

	event := hook.ParseHookInput(data)
	hook.SendToSocket(event)

	os.Exit(0)
}
