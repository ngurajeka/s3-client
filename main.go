package main

import (
	"fmt"
	"os"
	"strings"

	"s3-client/internal/cmd/download"
)

const binaryName = "s3-client"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	sub := strings.ToLower(strings.TrimSpace(os.Args[1]))
	args := os.Args[2:]

	switch sub {
	case "download", "dl":
		code := download.Run(args)
		os.Exit(code)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %q\n\n", sub)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\n", binaryName)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  download, dl    Download an object from S3 (parallel chunked)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "Use \"%s <command> -h\" for command-specific help.\n", binaryName)
}
