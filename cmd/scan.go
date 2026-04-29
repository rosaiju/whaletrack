// Package cmd implements the CLI commands for whaletrack.
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

// ScanCmd runs a one-shot scan of recent SEC Form 4 filings.
func ScanCmd(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	txType := fs.String("type", "purchases", "Transaction type: purchases, sales, or all")
	minValue := fs.Float64("min-value", 100000, "Minimum transaction value in USD")
	days := fs.Int("days", 30, "Look back N days")
	workers := fs.Int("workers", 20, "Number of concurrent fetch workers")
	outFile := fs.String("out", "", "Output JSON file path (optional)")
	cik := fs.String("cik", "", "Filter by company CIK (optional)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Set up context with signal handling for graceful shutdown.
	// If the user hits Ctrl+C, context cancellation propagates through
	// every goroutine in the pipeline, shutting them down cleanly.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := edgar.NewClient()
	fmt.Fprintf(os.Stderr, "Scanning SEC EDGAR Form 4 filings (last %d days)...\n", *days)

	// Fetch filing index
	var indices []edgar.FilingIndex
	var err error

	if *cik != "" {
		indices, err = client.FetchForm4sByCIK(ctx, *cik, *days)
	} else {
		indices, err = client.FetchRecentForm4s(ctx, *days)
	}
	if err != nil {
		return fmt.Errorf("fetch filing index: %w", err)
	}

	if len(indices) == 0 {
		fmt.Fprintln(os.Stderr, "No filings found.")
		return nil
	}

	// Run the pipeline with a progress indicator
	start := time.Now()
	opts := pipeline.FilterOpts{
		Type:     *txType,
		MinValue: *minValue,
	}

	result, err := pipeline.Run(ctx, client, indices, opts, *workers, func(processed, total int) {
		pct := float64(processed) / float64(total) * 100
		barLen := 32
		filled := int(pct / 100 * float64(barLen))
		bar := ""
		for i := 0; i < barLen; i++ {
			if i < filled {
				bar += "="
			} else {
				bar += " "
			}
		}
		fmt.Fprintf(os.Stderr, "\r[%s] %d/%d filings (%.0f%%)", bar, processed, total, pct)
	})
	if err != nil {
		return fmt.Errorf("pipeline: %w", err)
	}

	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "\r[================================] %d filings processed (%.1fs, %d workers)\n\n",
		result.Stats.FilingsTotal, elapsed.Seconds(), *workers)

	// Print header
	label := "Insider Transactions"
	switch *txType {
	case "purchases":
		label = "Insider Purchases"
	case "sales":
		label = "Insider Sales"
	}
	if *minValue > 0 {
		label += fmt.Sprintf(" > %s", formatValue(*minValue))
	}
	label += fmt.Sprintf(" (Last %d Days)", *days)
	fmt.Println(label)

	// Output table
	output.PrintTable(os.Stdout, result.Filings)

	// Optional JSON export
	if *outFile != "" {
		if err := output.WriteJSON(*outFile, result.Filings); err != nil {
			return fmt.Errorf("write JSON: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Saved to %s\n", *outFile)
	}

	return nil
}

func formatValue(v float64) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("$%.0fM", v/1_000_000)
	}
	if v >= 1_000 {
		return fmt.Sprintf("$%.0fK", v/1_000)
	}
	return fmt.Sprintf("$%.0f", v)
}
