package pipeline

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

// Stats tracks pipeline progress.
type Stats struct {
	FilingsTotal     int
	FilingsProcessed atomic.Int64
	ResultCount      int
}

// Result holds the final output of a pipeline run.
type Result struct {
	Filings []*edgar.Filing
	Stats   Stats
}

// Run orchestrates the full pipeline: index -> fetch -> parse/filter -> collect.
//
// Architecture:
//   1. Send all filing indices into a channel
//   2. Fetcher workers (fan-out) download XML concurrently
//   3. Processor parses XML and filters transactions
//   4. Collect results into a slice
//
// Context cancellation triggers graceful shutdown at every stage.
func Run(ctx context.Context, client *edgar.Client, indices []edgar.FilingIndex, opts FilterOpts, workers int, progress func(processed, total int)) (*Result, error) {
	if len(indices) == 0 {
		return &Result{}, nil
	}

	stats := Stats{FilingsTotal: len(indices)}

	// Stage 1: Feed filing indices into a channel.
	indexCh := make(chan edgar.FilingIndex, workers*2)
	go func() {
		defer close(indexCh)
		for _, idx := range indices {
			select {
			case <-ctx.Done():
				return
			case indexCh <- idx:
			}
		}
	}()

	// Stage 2: Fan-out fetch workers.
	fetched := Fetcher(ctx, client, indexCh, workers)

	// Wrap fetched to count progress.
	counted := make(chan fetchResult, cap(fetched))
	go func() {
		defer close(counted)
		for fr := range fetched {
			n := int(stats.FilingsProcessed.Add(1))
			if progress != nil {
				progress(n, stats.FilingsTotal)
			}
			counted <- fr
		}
	}()

	// Stage 3: Parse and filter.
	processed := Processor(ctx, counted, opts)

	// Stage 4: Collect results.
	var filings []*edgar.Filing
	seen := make(map[string]bool) // deduplicate by owner+ticker+date
	for filing := range processed {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("cancelled: %w", ctx.Err())
		}
		key := dedupeKey(filing)
		if seen[key] {
			continue
		}
		seen[key] = true
		filings = append(filings, filing)
	}

	stats.ResultCount = len(filings)
	return &Result{Filings: filings, Stats: stats}, nil
}

func dedupeKey(f *edgar.Filing) string {
	if len(f.Transactions) == 0 {
		return f.Owner.CIK + "|" + f.Issuer.Ticker
	}
	return fmt.Sprintf("%s|%s|%s", f.Owner.CIK, f.Issuer.Ticker, f.Transactions[0].Date.Format("2006-01-02"))
}
