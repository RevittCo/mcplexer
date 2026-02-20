package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcplexer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse subcommand from os.Args
	subcmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "serve":
		return cmdServe(args)
	case "connect":
		return cmdConnect(args)
	case "init":
		return cmdInit()
	case "status":
		return cmdStatus()
	case "dry-run":
		return cmdDryRun(args)
	case "secret":
		return cmdSecret(args)
	case "daemon":
		return cmdDaemon(args)
	case "setup":
		return cmdSetup()
	case "control-server":
		return cmdControlServer()
	default:
		return fmt.Errorf("unknown command: %s\nUsage: mcplexer [serve|connect|init|status|dry-run|secret|daemon|setup|control-server]", subcmd)
	}
}
