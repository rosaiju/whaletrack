package edgar

import (
	"testing"
	"time"
)

// Sample Form 4 XML based on real SEC filing format.
const sampleForm4XML = `<?xml version="1.0" encoding="UTF-8"?>
<ownershipDocument>
    <issuer>
        <issuerCik>0001326801</issuerCik>
        <issuerName>Meta Platforms, Inc.</issuerName>
        <issuerTradingSymbol>META</issuerTradingSymbol>
    </issuer>
    <reportingOwner>
        <reportingOwnerId>
            <rptOwnerCik>0001548760</rptOwnerCik>
            <rptOwnerName>Sandberg Sheryl</rptOwnerName>
        </reportingOwnerId>
        <reportingOwnerRelationship>
            <isDirector>1</isDirector>
            <isOfficer>0</isOfficer>
            <officerTitle></officerTitle>
            <isTenPercentOwner>0</isTenPercentOwner>
        </reportingOwnerRelationship>
    </reportingOwner>
    <nonDerivativeTable>
        <nonDerivativeTransaction>
            <securityTitle><value>Class A Common Stock</value></securityTitle>
            <transactionDate><value>2026-04-15</value></transactionDate>
            <transactionCoding>
                <transactionCode>P</transactionCode>
            </transactionCoding>
            <transactionAmounts>
                <transactionShares><value>10000</value></transactionShares>
                <transactionPricePerShare><value>525.50</value></transactionPricePerShare>
                <transactionAcquiredDisposedCode><value>A</value></transactionAcquiredDisposedCode>
            </transactionAmounts>
            <postTransactionAmounts>
                <sharesOwnedFollowingTransaction><value>150000</value></sharesOwnedFollowingTransaction>
            </postTransactionAmounts>
        </nonDerivativeTransaction>
        <nonDerivativeTransaction>
            <securityTitle><value>Class A Common Stock</value></securityTitle>
            <transactionDate><value>2026-04-16</value></transactionDate>
            <transactionCoding>
                <transactionCode>S</transactionCode>
            </transactionCoding>
            <transactionAmounts>
                <transactionShares><value>5000</value></transactionShares>
                <transactionPricePerShare><value>530.00</value></transactionPricePerShare>
                <transactionAcquiredDisposedCode><value>D</value></transactionAcquiredDisposedCode>
            </transactionAmounts>
            <postTransactionAmounts>
                <sharesOwnedFollowingTransaction><value>145000</value></sharesOwnedFollowingTransaction>
            </postTransactionAmounts>
        </nonDerivativeTransaction>
    </nonDerivativeTable>
</ownershipDocument>`

func TestParseForm4(t *testing.T) {
	filed := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	filing, err := ParseForm4([]byte(sampleForm4XML), "https://sec.gov/test", filed)
	if err != nil {
		t.Fatalf("ParseForm4 failed: %v", err)
	}

	// Verify issuer
	if filing.Issuer.Ticker != "META" {
		t.Errorf("expected ticker META, got %s", filing.Issuer.Ticker)
	}
	if filing.Issuer.Name != "Meta Platforms, Inc." {
		t.Errorf("expected issuer name 'Meta Platforms, Inc.', got %s", filing.Issuer.Name)
	}

	// Verify owner
	if filing.Owner.Name != "Sandberg Sheryl" {
		t.Errorf("expected owner 'Sandberg Sheryl', got %s", filing.Owner.Name)
	}
	if !filing.Owner.IsDirector {
		t.Error("expected IsDirector to be true")
	}
	if filing.Owner.IsOfficer {
		t.Error("expected IsOfficer to be false")
	}

	// Verify transactions
	if len(filing.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(filing.Transactions))
	}

	buy := filing.Transactions[0]
	if !buy.IsPurchase() {
		t.Error("expected first transaction to be a purchase")
	}
	if buy.Shares != 10000 {
		t.Errorf("expected 10000 shares, got %.0f", buy.Shares)
	}
	if buy.PricePerShare != 525.50 {
		t.Errorf("expected price 525.50, got %.2f", buy.PricePerShare)
	}
	if buy.TotalValue != 5255000 {
		t.Errorf("expected total value 5255000, got %.2f", buy.TotalValue)
	}
	if buy.SharesOwnedPost != 150000 {
		t.Errorf("expected post-transaction shares 150000, got %.0f", buy.SharesOwnedPost)
	}

	sell := filing.Transactions[1]
	if !sell.IsSale() {
		t.Error("expected second transaction to be a sale")
	}
	if sell.Shares != 5000 {
		t.Errorf("expected 5000 shares, got %.0f", sell.Shares)
	}
}

func TestParseForm4_WrongRootElement(t *testing.T) {
	_, err := ParseForm4([]byte("<not-a-form4/>"), "https://sec.gov/test", time.Now())
	if err == nil {
		t.Fatal("expected error on non-Form4 XML root element")
	}
}

func TestParseForm4_EmptyDocument(t *testing.T) {
	xml := `<ownershipDocument>
		<issuer><issuerCik>123</issuerCik><issuerName>Test</issuerName><issuerTradingSymbol>TST</issuerTradingSymbol></issuer>
		<reportingOwner><reportingOwnerId><rptOwnerCik>456</rptOwnerCik><rptOwnerName>Test Owner</rptOwnerName></reportingOwnerId>
		<reportingOwnerRelationship><isDirector>0</isDirector><isOfficer>0</isOfficer><isTenPercentOwner>0</isTenPercentOwner></reportingOwnerRelationship></reportingOwner>
	</ownershipDocument>`
	filing, err := ParseForm4([]byte(xml), "https://sec.gov/test", time.Now())
	if err != nil {
		t.Fatalf("expected no error on empty Form 4, got: %v", err)
	}
	if len(filing.Transactions) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(filing.Transactions))
	}
	if filing.Issuer.Ticker != "TST" {
		t.Errorf("expected ticker TST, got %s", filing.Issuer.Ticker)
	}
}

func TestParseForm4_MalformedXML(t *testing.T) {
	_, err := ParseForm4([]byte("this is not xml at all"), "", time.Now())
	if err == nil {
		t.Error("expected error on malformed XML")
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"100.50", 100.50},
		{"1,000", 1000},
		{"  50  ", 50},
		{"", 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		got := parseFloat(tt.input)
		if got != tt.expected {
			t.Errorf("parseFloat(%q) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"True", true},
		{"yes", true},
		{"0", false},
		{"false", false},
		{"", false},
	}
	for _, tt := range tests {
		got := parseBool(tt.input)
		if got != tt.expected {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
