package edgar

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// xmlOwnershipDocument is the root element of a Form 4 XML filing.
// SEC XML is inconsistent, so we use lenient parsing with pointers and omitempty.
type xmlOwnershipDocument struct {
	XMLName       xml.Name           `xml:"ownershipDocument"`
	Issuer        xmlIssuer          `xml:"issuer"`
	Owner         xmlOwner           `xml:"reportingOwner"`
	NonDerivTable *xmlNonDerivTable  `xml:"nonDerivativeTable"`
	DerivTable    *xmlDerivTable     `xml:"derivativeTable"`
}

type xmlIssuer struct {
	CIK    string `xml:"issuerCik"`
	Name   string `xml:"issuerName"`
	Ticker string `xml:"issuerTradingSymbol"`
}

type xmlOwner struct {
	OwnerID  xmlOwnerID  `xml:"reportingOwnerId"`
	OwnerRel xmlOwnerRel `xml:"reportingOwnerRelationship"`
}

type xmlOwnerID struct {
	CIK  string `xml:"rptOwnerCik"`
	Name string `xml:"rptOwnerName"`
}

type xmlOwnerRel struct {
	IsDirector   string `xml:"isDirector"`
	IsOfficer    string `xml:"isOfficer"`
	OfficerTitle string `xml:"officerTitle"`
	IsTenPercent string `xml:"isTenPercentOwner"`
}

type xmlNonDerivTable struct {
	Transactions []xmlNonDerivTransaction `xml:"nonDerivativeTransaction"`
}

type xmlDerivTable struct {
	Transactions []xmlDerivTransaction `xml:"derivativeTransaction"`
}

type xmlNonDerivTransaction struct {
	SecurityTitle xmlValue   `xml:"securityTitle"`
	TransDate     xmlValue   `xml:"transactionDate"`
	Coding        xmlCoding  `xml:"transactionCoding"`
	Amounts       xmlAmounts `xml:"transactionAmounts"`
	PostShares    xmlValue   `xml:"postTransactionAmounts>sharesOwnedFollowingTransaction"`
}

type xmlDerivTransaction struct {
	SecurityTitle xmlValue      `xml:"securityTitle"`
	ConvDate      xmlValue      `xml:"conversionOrExercisePrice"`
	TransDate     xmlValue      `xml:"transactionDate"`
	Coding        xmlCoding     `xml:"transactionCoding"`
	Amounts       xmlAmounts    `xml:"transactionAmounts"`
	UnderlyingSec xmlUnderlying `xml:"underlyingSecurity"`
}

type xmlUnderlying struct {
	Title  xmlValue `xml:"underlyingSecurityTitle"`
	Shares xmlValue `xml:"underlyingSecurityShares"`
}

type xmlValue struct {
	Value string `xml:"value"`
}

type xmlCoding struct {
	Code string `xml:"transactionCode"`
}

type xmlAmounts struct {
	Shares        xmlValue `xml:"transactionShares"`
	PricePerShare xmlValue `xml:"transactionPricePerShare"`
	AcqOrDisp     xmlValue `xml:"transactionAcquiredDisposedCode"`
}

// ParseForm4 parses raw Form 4 XML into a Filing struct.
func ParseForm4(data []byte, filingURL string, filedAt time.Time) (*Filing, error) {
	var doc xmlOwnershipDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal XML: %w", err)
	}

	filing := &Filing{
		Issuer: Issuer{
			CIK:    strings.TrimSpace(doc.Issuer.CIK),
			Name:   strings.TrimSpace(doc.Issuer.Name),
			Ticker: strings.ToUpper(strings.TrimSpace(doc.Issuer.Ticker)),
		},
		Owner: Owner{
			CIK:          strings.TrimSpace(doc.Owner.OwnerID.CIK),
			Name:         strings.TrimSpace(doc.Owner.OwnerID.Name),
			IsDirector:   parseBool(doc.Owner.OwnerRel.IsDirector),
			IsOfficer:    parseBool(doc.Owner.OwnerRel.IsOfficer),
			OfficerTitle: strings.TrimSpace(doc.Owner.OwnerRel.OfficerTitle),
			IsTenPercent: parseBool(doc.Owner.OwnerRel.IsTenPercent),
		},
		FiledAt: filedAt,
		URL:     filingURL,
	}

	// Parse non-derivative transactions (direct stock buys/sells)
	if doc.NonDerivTable != nil {
		for _, xt := range doc.NonDerivTable.Transactions {
			t, err := parseNonDerivTransaction(xt)
			if err != nil {
				continue // skip malformed transactions rather than failing
			}
			filing.Transactions = append(filing.Transactions, t)
		}
	}

	return filing, nil
}

func parseNonDerivTransaction(xt xmlNonDerivTransaction) (Transaction, error) {
	shares := parseFloat(xt.Amounts.Shares.Value)
	price := parseFloat(xt.Amounts.PricePerShare.Value)
	postShares := parseFloat(xt.PostShares.Value)

	date, err := time.Parse("2006-01-02", strings.TrimSpace(xt.TransDate.Value))
	if err != nil {
		return Transaction{}, fmt.Errorf("parse date %q: %w", xt.TransDate.Value, err)
	}

	return Transaction{
		Date:            date,
		Code:            strings.TrimSpace(xt.Coding.Code),
		Shares:          shares,
		PricePerShare:   price,
		TotalValue:      math.Round(shares*price*100) / 100,
		AcquiredOrSold:  strings.TrimSpace(xt.Amounts.AcqOrDisp.Value),
		SharesOwnedPost: postShares,
	}, nil
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes"
}
