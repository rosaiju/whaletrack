// whaletrack scans SEC EDGAR for insider trading activity.
//
// Usage:
//
//	whaletrack scan [flags]    One-shot scan of recent Form 4 filings
//	whaletrack watch [flags]   Continuous monitoring mode
package main

import (
	"fmt"
	"os"

	"github.com/rosaiju/whaletrack/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "scan":
		err = cmd.ScanCmd(os.Args[2:])
	case "watch":
		err = cmd.WatchCmd(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `whaletrack — SEC EDGAR insider trading scanner

Usage:
  whaletrack scan [flags]    Scan recent Form 4 filings
  whaletrack watch [flags]   Continuous monitoring mode

Scan flags:
  -type string       Transaction type: purchases, sales, all (default "purchases")
  -min-value float   Minimum transaction value in USD (default 100000)
  -days int          Look back N days (default 30)
  -workers int       Concurrent fetch workers (default 20)
  -cik string        Filter by company CIK
  -out string        Export results to JSON file

Watch flags:
  -type string       Transaction type (default "purchases")
  -min-value float   Minimum value (default 500000)
  -interval duration Time between scans (default 15m)
  -workers int       Concurrent workers (default 20)
  -out string        Export to JSON (overwritten each scan)

Examples:
  whaletrack scan --type purchases --min-value 500000 --days 30
  whaletrack scan --cik 1326801 --days 90
  whaletrack watch --min-value 1000000 --interval 10m
`)
}
