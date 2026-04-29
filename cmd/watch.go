package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/rosaiju/whaletrack/internal/edgar"
	"github.com/rosaiju/whaletrack/internal/output"
	"github.com/rosaiju/whaletrack/internal/pipeline"
)

// WatchCmd runs continuous monitoring, re-scanning at a fixed interval.
func WatchCmd(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	txType := fs.String("type", "purchases", "Transaction type: purchases, sales, or all")
	minValue := fs.Float64("min-value", 500000, "Minimum transaction value in USD")
	interval := fs.Duration("interval", 15*time.Minute, "Time between scans")
	workers := fs.Int("workers", 20, "Number of concurrent fetch workers")
	outFile := fs.String("out", "", "Output JSON file (overwritten each scan)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := edgar.NewClient()
	opts := pipeline.FilterOpts{
		Type:     *txType,
		MinValue: *minValue,
	}

	fmt.Fprintf(os.Stderr, "Watching for insider %s > %s (every %s). Press Ctrl+C to stop.\n\n",
		*txType, formatValue(*minValue), *interval)

	// Track filings we've already seen to only show new ones
	seen := make(map[string]bool)
	scanNum := 0

	for {
		scanNum++
		fmt.Fprintf(os.Stderr, "--- Scan #%d at %s ---\n", scanNum, time.Now().Format("15:04:05"))

		indices, err := client.FetchRecentForm4s(ctx, 1) // last 24 hours
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching index: %v\n", err)
		} else {
			result, err := pipeline.Run(ctx, client, indices, opts, *workers, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Pipeline error: %v\n", err)
			} else {
				// Filter to only new filings
				var newFilings []*edgar.Filing
				for _, f := range result.Filings {
					key := f.Owner.CIK + "|" + f.Issuer.Ticker
					if !seen[key] {
						seen[key] = true
						newFilings = append(newFilings, f)
					}
				}

				if len(newFilings) > 0 {
					fmt.Printf("\n%d new filing(s) found:\n", len(newFilings))
					output.PrintTable(os.Stdout, newFilings)
					if *outFile != "" {
						output.WriteJSON(*outFile, result.Filings)
					}
				} else {
					fmt.Fprintf(os.Stderr, "No new filings.\n")
				}
			}
		}

		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "\nStopped.")
			return nil
		case <-time.After(*interval):
		}
	}
}
