// Package edgar provides types and functions for working with SEC EDGAR filings.
package edgar

import "time"

// Filing represents a parsed SEC Form 4 filing.
type Filing struct {
	AccessionNumber string        `json:"accession_number"`
	Issuer          Issuer        `json:"issuer"`
	Owner           Owner         `json:"owner"`
	Transactions    []Transaction `json:"transactions"`
	FiledAt         time.Time     `json:"filed_at"`
	URL             string        `json:"url"`
}

// Issuer is the company whose stock was traded.
type Issuer struct {
	CIK    string `json:"cik"`
	Name   string `json:"name"`
	Ticker string `json:"ticker"`
}

// Owner is the insider who made the trade.
type Owner struct {
	CIK          string `json:"cik"`
	Name         string `json:"name"`
	IsDirector   bool   `json:"is_director"`
	IsOfficer    bool   `json:"is_officer"`
	OfficerTitle string `json:"officer_title,omitempty"`
	IsTenPercent bool   `json:"is_ten_percent_owner"`
}

// Transaction represents a single buy or sell within a Form 4 filing.
type Transaction struct {
	Date            time.Time `json:"date"`
	Code            string    `json:"code"`             // "P" = purchase, "S" = sale
	Shares          float64   `json:"shares"`           // number of shares
	PricePerShare   float64   `json:"price_per_share"`  // price per share in USD
	TotalValue      float64   `json:"total_value"`      // shares * price
	AcquiredOrSold  string    `json:"acquired_or_sold"` // "A" = acquired, "D" = disposed
	SharesOwnedPost float64  `json:"shares_owned_post"` // shares owned after transaction
}

// TransactionCode descriptions for display.
var TransactionCodes = map[string]string{
	"P": "Purchase",
	"S": "Sale",
	"A": "Grant/Award",
	"M": "Option Exercise",
	"G": "Gift",
	"F": "Tax Withholding",
	"C": "Conversion",
}

// IsPurchase returns true if this transaction is an open-market purchase.
func (t Transaction) IsPurchase() bool {
	return t.Code == "P"
}

// IsSale returns true if this transaction is an open-market sale.
func (t Transaction) IsSale() bool {
	return t.Code == "S"
}

// FilingIndex represents an entry from the EDGAR full-text search or RSS feed.
type FilingIndex struct {
	AccessionNumber string
	CIK             string
	FiledAt         time.Time
	URL             string
}
