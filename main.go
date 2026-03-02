// Yao Registry — a lightweight package registry for Yao assets.
package main

import (
	"fmt"
	"os"

	"github.com/yaoapp/registry/cmd"
	"github.com/yaoapp/registry/handlers"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		cmd.RunStart(nil)
		return
	}

	switch args[0] {
	case "start":
		cmd.RunStart(args[1:])
	case "user":
		cmd.RunUser(args[1:])
	case "version":
		fmt.Println("yao-registry " + handlers.Version)
	case "help", "-h", "--help":
		printUsage()
	default:
		// If the first arg looks like a flag, treat as "start" with flags
		if len(args[0]) > 0 && args[0][0] == '-' {
			cmd.RunStart(args)
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: registry <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  start    Start the registry server (default)")
	fmt.Fprintln(os.Stderr, "  user     Manage push credentials")
	fmt.Fprintln(os.Stderr, "  version  Print version")
	fmt.Fprintln(os.Stderr, "  help     Show this help")
}
