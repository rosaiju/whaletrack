package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

func makeFiling(ticker, owner, title string, shares, price float64) *edgar.Filing {
	return &edgar.Filing{
		Issuer: edgar.Issuer{Ticker: ticker, Name: ticker + " Inc."},
		Owner:  edgar.Owner{Name: owner, OfficerTitle: title},
		Transactions: []edgar.Transaction{
			{
				Date:          time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
				Code:          "P",
				Shares:        shares,
				PricePerShare: price,
				TotalValue:    shares * price,
			},
		},
	}
}

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	PrintTable(&buf, nil)

	if !strings.Contains(buf.String(), "No matching filings found.") {
		t.Errorf("expected empty message, got: %s", buf.String())
	}
}

func TestPrintTable_SingleFiling(t *testing.T) {
	var buf bytes.Buffer
	filings := []*edgar.Filing{
		makeFiling("AAPL", "Tim Cook", "CEO", 50000, 180.00),
	}
	PrintTable(&buf, filings)

	out := buf.String()
	if !strings.Contains(out, "AAPL") {
		t.Error("expected output to contain ticker AAPL")
	}
	if !strings.Contains(out, "Tim Cook") {
		t.Error("expected output to contain insider name")
	}
	if !strings.Contains(out, "CEO") {
		t.Error("expected output to contain title")
	}
	if !strings.Contains(out, "$9.0M") {
		t.Errorf("expected $9.0M total value, got: %s", out)
	}
	if !strings.Contains(out, "1 results.") {
		t.Errorf("expected '1 results.' footer, got: %s", out)
	}
}

func TestPrintTable_SortsByValueDescending(t *testing.T) {
	var buf bytes.Buffer
	filings := []*edgar.Filing{
		makeFiling("SMALL", "Small Buyer", "", 100, 10.00),     // $1K
		makeFiling("BIG", "Big Buyer", "CFO", 100000, 500.00),  // $50M
		makeFiling("MID", "Mid Buyer", "VP", 10000, 100.00),    // $1M
	}
	PrintTable(&buf, filings)

	out := buf.String()
	bigIdx := strings.Index(out, "BIG")
	midIdx := strings.Index(out, "MID")
	smallIdx := strings.Index(out, "SMALL")

	if bigIdx > midIdx || midIdx > smallIdx {
		t.Error("expected rows sorted by value descending: BIG, MID, SMALL")
	}
}

func TestPrintTable_TruncatesLongNames(t *testing.T) {
	var buf bytes.Buffer
	longName := "Bartholomew Jedediah Winterbottom III"
	filings := []*edgar.Filing{
		makeFiling("TEST", longName, "CEO", 1000, 100.00),
	}
	PrintTable(&buf, filings)

	out := buf.String()
	// Name should be truncated (max 25 chars with trailing '.')
	if strings.Contains(out, longName) {
		t.Error("expected long insider name to be truncated")
	}
}

func TestPrintTable_DirectorTitle(t *testing.T) {
	var buf bytes.Buffer
	f := makeFiling("META", "Jane Doe", "", 5000, 500.00)
	f.Owner.OfficerTitle = ""
	f.Owner.IsDirector = true
	PrintTable(&buf, []*edgar.Filing{f})

	if !strings.Contains(buf.String(), "Director") {
		t.Error("expected Director title fallback for director owner")
	}
}

func TestPrintTable_TenPercentOwnerTitle(t *testing.T) {
	var buf bytes.Buffer
	f := makeFiling("TSLA", "Big Fund", "", 100000, 200.00)
	f.Owner.OfficerTitle = ""
	f.Owner.IsDirector = false
	f.Owner.IsTenPercent = true
	PrintTable(&buf, []*edgar.Filing{f})

	if !strings.Contains(buf.String(), "10% Owner") {
		t.Error("expected '10% Owner' title fallback")
	}
}

func TestFormatDollar(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{5_255_000, "$5.3M"},
		{1_000_000, "$1.0M"},
		{500_000, "$500K"},
		{1_500, "$2K"},
		{999, "$999"},
		{50, "$50"},
		{0, "$0"},
	}
	for _, tt := range tests {
		got := formatDollar(tt.input)
		if got != tt.expected {
			t.Errorf("formatDollar(%f) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatShares(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{2_500_000, "2.5M"},
		{1_000_000, "1.0M"},
		{50_000, "50K"},
		{1_500, "2K"},
		{999, "999"},
		{0, "0"},
	}
	for _, tt := range tests {
		got := formatShares(tt.input)
		if got != tt.expected {
			t.Errorf("formatShares(%f) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
