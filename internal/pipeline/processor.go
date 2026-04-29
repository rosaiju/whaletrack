package pipeline

import (
	"context"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

// FilterOpts controls which transactions make it through the pipeline.
type FilterOpts struct {
	Type     string  // "purchases", "sales", or "" for all
	MinValue float64 // minimum transaction value in USD
}

// Processor reads raw XML from the fetcher stage, parses each filing,
// filters transactions by the given criteria, and emits matching filings.
func Processor(ctx context.Context, fetched <-chan fetchResult, opts FilterOpts) <-chan *edgar.Filing {
	results := make(chan *edgar.Filing, 32)

	go func() {
		defer close(results)
		for fr := range fetched {
			if ctx.Err() != nil {
				return
			}
			if fr.Err != nil {
				continue // skip failed fetches
			}

			filing, err := edgar.ParseForm4(fr.Data, fr.Index.URL, fr.Index.FiledAt)
			if err != nil {
				continue // skip unparseable filings
			}

			// Filter transactions
			var kept []edgar.Transaction
			for _, t := range filing.Transactions {
				if !matchesFilter(t, opts) {
					continue
				}
				kept = append(kept, t)
			}

			if len(kept) == 0 {
				continue
			}

			filing.Transactions = kept
			results <- filing
		}
	}()

	return results
}

func matchesFilter(t edgar.Transaction, opts FilterOpts) bool {
	switch opts.Type {
	case "purchases":
		if !t.IsPurchase() {
			return false
		}
	case "sales":
		if !t.IsSale() {
			return false
		}
	}

	if opts.MinValue > 0 && t.TotalValue < opts.MinValue {
		return false
	}

	return true
}
