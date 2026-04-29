package pipeline

import (
	"testing"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

func TestMatchesFilter_Purchases(t *testing.T) {
	opts := FilterOpts{Type: "purchases", MinValue: 100000}

	buy := edgar.Transaction{Code: "P", TotalValue: 500000}
	if !matchesFilter(buy, opts) {
		t.Error("expected purchase to match")
	}

	sell := edgar.Transaction{Code: "S", TotalValue: 500000}
	if matchesFilter(sell, opts) {
		t.Error("expected sale to NOT match purchases filter")
	}

	smallBuy := edgar.Transaction{Code: "P", TotalValue: 50000}
	if matchesFilter(smallBuy, opts) {
		t.Error("expected small purchase to NOT match min-value filter")
	}
}

func TestMatchesFilter_Sales(t *testing.T) {
	opts := FilterOpts{Type: "sales", MinValue: 0}

	sell := edgar.Transaction{Code: "S", TotalValue: 100}
	if !matchesFilter(sell, opts) {
		t.Error("expected sale to match")
	}

	buy := edgar.Transaction{Code: "P", TotalValue: 100}
	if matchesFilter(buy, opts) {
		t.Error("expected purchase to NOT match sales filter")
	}
}

func TestMatchesFilter_All(t *testing.T) {
	opts := FilterOpts{Type: "", MinValue: 0}

	buy := edgar.Transaction{Code: "P", TotalValue: 100}
	sell := edgar.Transaction{Code: "S", TotalValue: 100}
	grant := edgar.Transaction{Code: "A", TotalValue: 100}

	if !matchesFilter(buy, opts) || !matchesFilter(sell, opts) || !matchesFilter(grant, opts) {
		t.Error("expected all transaction types to match with empty type filter")
	}
}

func TestDedupeKey(t *testing.T) {
	f1 := &edgar.Filing{
		Owner:  edgar.Owner{CIK: "123"},
		Issuer: edgar.Issuer{Ticker: "AAPL"},
		Transactions: []edgar.Transaction{
			{Code: "P"},
		},
	}

	f2 := &edgar.Filing{
		Owner:  edgar.Owner{CIK: "123"},
		Issuer: edgar.Issuer{Ticker: "AAPL"},
		Transactions: []edgar.Transaction{
			{Code: "P"},
		},
	}

	if dedupeKey(f1) != dedupeKey(f2) {
		t.Error("identical filings should have the same dedupe key")
	}

	f3 := &edgar.Filing{
		Owner:  edgar.Owner{CIK: "456"},
		Issuer: edgar.Issuer{Ticker: "AAPL"},
		Transactions: []edgar.Transaction{
			{Code: "P"},
		},
	}

	if dedupeKey(f1) == dedupeKey(f3) {
		t.Error("different owners should have different dedupe keys")
	}
}
