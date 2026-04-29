// Package pipeline implements a concurrent fan-out/fan-in pipeline
// for fetching and processing SEC EDGAR filings.
package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

// fetchResult carries raw XML data or an error from a fetch worker.
type fetchResult struct {
	Index edgar.FilingIndex
	Data  []byte
	Err   error
}

// Fetcher spawns multiple goroutines to download filing XML concurrently.
// It reads filing indices from the input channel, fetches each one, and
// sends the raw XML (or error) to the output channel.
//
// Fan-out pattern: `workers` goroutines all read from the same input channel.
// Go's channel semantics ensure each filing is processed by exactly one worker.
func Fetcher(ctx context.Context, client *edgar.Client, indices <-chan edgar.FilingIndex, workers int) <-chan fetchResult {
	results := make(chan fetchResult, workers*2) // buffered to prevent blocking

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for idx := range indices {
				if ctx.Err() != nil {
					return
				}

				data, err := client.FetchFilingXML(ctx, idx.URL)
				if err != nil {
					results <- fetchResult{
						Index: idx,
						Err:   fmt.Errorf("worker %d: fetch %s: %w", workerID, idx.AccessionNumber, err),
					}
					continue
				}

				results <- fetchResult{
					Index: idx,
					Data:  data,
				}
			}
		}(i)
	}

	// Close results channel once all workers are done (fan-in).
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}
