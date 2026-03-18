// Package main is the entry point for the highlights exporter service.
package main

import (
	"fmt"
	"os"

	"github.com/mrlokans/assistant/internal/cli"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entrypoint"
)

// Version information - set at build time via ldflags
var (
	Version = "dev"
	Commit  = "unknown"
)

type cliCommand interface {
	ParseFlags(args []string) error
	Run() error
}

func runCommand(cmd cliCommand, args []string) {
	if err := cmd.ParseFlags(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	// If no arguments or "serve" command, run the HTTP server
	if len(os.Args) < 2 || os.Args[1] == "serve" {
		cfg := config.NewConfig()
		entrypoint.Run(cfg, Version)
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "moonreader-sync":
		runCommand(cli.NewMoonReaderSyncCommand(), args)
	case "moonreader-dropbox":
		runCommand(cli.NewMoonReaderDropboxCommand(), args)
	case "dropbox-auth":
		runCommand(cli.NewDropboxAuthCommand(), args)
	case "parse-markdown":
		runCommand(cli.NewParseMarkdownCommand(), args)
	case "applebooks-import":
		runCommand(cli.NewAppleBooksImportCommand(), args)
	case "kindle-import":
		runCommand(cli.NewKindleImportCommand(), args)
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  serve               Start the HTTP server (default if no command given)\n")
	fmt.Fprintf(os.Stderr, "  moonreader-sync     Sync MoonReader highlights from local filesystem\n")
	fmt.Fprintf(os.Stderr, "  moonreader-dropbox  Sync MoonReader highlights from Dropbox\n")
	fmt.Fprintf(os.Stderr, "  dropbox-auth        Perform Dropbox OAuth flow to get access token\n")
	fmt.Fprintf(os.Stderr, "  parse-markdown      Parse markdown files recursively from a directory\n")
	fmt.Fprintf(os.Stderr, "  applebooks-import   Import highlights from Apple Books (macOS only)\n")
	fmt.Fprintf(os.Stderr, "  kindle-import       Import highlights from Kindle 'My Clippings.txt'\n")
	fmt.Fprintf(os.Stderr, "\nUse '%s <command> -h' for help on a specific command.\n", os.Args[0])
}
