// Package output formats pipeline results for display.
package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

// PrintTable writes a formatted terminal table of filings.
// Aggregates transactions per filing into a single row showing total value.
func PrintTable(w io.Writer, filings []*edgar.Filing) {
	if len(filings) == 0 {
		fmt.Fprintln(w, "No matching filings found.")
		return
	}

	type row struct {
		Ticker  string
		Insider string
		Title   string
		Amount  float64
		Shares  float64
		Date    string
	}

	var rows []row
	for _, f := range filings {
		var totalShares, totalValue float64
		var latestDate string
		for _, t := range f.Transactions {
			totalShares += t.Shares
			totalValue += t.TotalValue
			latestDate = t.Date.Format("2006-01-02")
		}

		title := f.Owner.OfficerTitle
		if title == "" {
			if f.Owner.IsDirector {
				title = "Director"
			} else if f.Owner.IsTenPercent {
				title = "10% Owner"
			}
		}

		rows = append(rows, row{
			Ticker:  f.Issuer.Ticker,
			Insider: f.Owner.Name,
			Title:   title,
			Amount:  totalValue,
			Shares:  totalShares,
			Date:    latestDate,
		})
	}

	// Sort by total value descending
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Amount > rows[j].Amount
	})

	// Calculate column widths
	tickerW, insiderW, titleW, amountW, sharesW := 6, 7, 5, 8, 6
	for _, r := range rows {
		if len(r.Ticker) > tickerW {
			tickerW = len(r.Ticker)
		}
		if len(r.Insider) > insiderW {
			insiderW = len(r.Insider)
		}
		if len(r.Title) > titleW {
			titleW = len(r.Title)
		}
		a := formatDollar(r.Amount)
		if len(a) > amountW {
			amountW = len(a)
		}
		s := formatShares(r.Shares)
		if len(s) > sharesW {
			sharesW = len(s)
		}
	}
	// Cap insider name width
	if insiderW > 25 {
		insiderW = 25
	}

	// Print header
	sep := "+" + strings.Repeat("-", tickerW+2) +
		"+" + strings.Repeat("-", insiderW+2) +
		"+" + strings.Repeat("-", titleW+2) +
		"+" + strings.Repeat("-", amountW+2) +
		"+" + strings.Repeat("-", sharesW+2) +
		"+" + strings.Repeat("-", 12) + "+"

	fmt.Fprintln(w, sep)
	fmt.Fprintf(w, "| %-*s | %-*s | %-*s | %-*s | %-*s | %-10s |\n",
		tickerW, "Ticker",
		insiderW, "Insider",
		titleW, "Title",
		amountW, "Amount",
		sharesW, "Shares",
		"Filed",
	)
	fmt.Fprintln(w, sep)

	for _, r := range rows {
		insider := r.Insider
		if len(insider) > insiderW {
			insider = insider[:insiderW-1] + "."
		}
		title := r.Title
		if len(title) > titleW {
			title = title[:titleW-1] + "."
		}
		fmt.Fprintf(w, "| %-*s | %-*s | %-*s | %*s | %*s | %-10s |\n",
			tickerW, r.Ticker,
			insiderW, insider,
			titleW, title,
			amountW, formatDollar(r.Amount),
			sharesW, formatShares(r.Shares),
			r.Date,
		)
	}

	fmt.Fprintln(w, sep)
	fmt.Fprintf(w, "\n%d results.\n", len(rows))
}

func formatDollar(v float64) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("$%.1fM", v/1_000_000)
	}
	if v >= 1_000 {
		return fmt.Sprintf("$%.0fK", v/1_000)
	}
	return fmt.Sprintf("$%.0f", v)
}

func formatShares(s float64) string {
	if s >= 1_000_000 {
		return fmt.Sprintf("%.1fM", s/1_000_000)
	}
	if s >= 1_000 {
		return fmt.Sprintf("%.0fK", s/1_000)
	}
	return fmt.Sprintf("%.0f", s)
}
