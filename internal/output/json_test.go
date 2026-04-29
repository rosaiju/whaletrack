package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rosaiju/whaletrack/internal/edgar"
)

func TestWriteJSON_CreatesValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	filings := []*edgar.Filing{
		{
			AccessionNumber: "0001234-26-000001",
			Issuer:          edgar.Issuer{CIK: "123", Name: "Test Corp", Ticker: "TST"},
			Owner:           edgar.Owner{CIK: "456", Name: "John Doe", IsOfficer: true, OfficerTitle: "CEO"},
			Transactions: []edgar.Transaction{
				{
					Date:          time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
					Code:          "P",
					Shares:        10000,
					PricePerShare: 50.00,
					TotalValue:    500000,
				},
			},
		},
	}

	if err := WriteJSON(path, filings); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var parsed []*edgar.Filing
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 filing, got %d", len(parsed))
	}
	if parsed[0].Issuer.Ticker != "TST" {
		t.Errorf("expected ticker TST, got %s", parsed[0].Issuer.Ticker)
	}
	if parsed[0].Owner.Name != "John Doe" {
		t.Errorf("expected owner John Doe, got %s", parsed[0].Owner.Name)
	}
}

func TestWriteJSON_EmptyFilings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")

	if err := WriteJSON(path, []*edgar.Filing{}); err != nil {
		t.Fatalf("WriteJSON failed on empty slice: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var parsed []*edgar.Filing
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(parsed) != 0 {
		t.Errorf("expected 0 filings, got %d", len(parsed))
	}
}

func TestWriteJSON_NilFilings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nil.json")

	if err := WriteJSON(path, nil); err != nil {
		t.Fatalf("WriteJSON failed on nil slice: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if string(data) != "null" {
		t.Errorf("expected 'null' for nil input, got %s", string(data))
	}
}
