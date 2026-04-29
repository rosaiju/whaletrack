package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rosaiju/whaletrack/internal/ratelimit"
)

const (
	// SEC requires a descriptive User-Agent with contact info.
	userAgent = "whaletrack/1.0 (github.com/rosaiju/whaletrack)"

	// EDGAR full-text search API endpoint.
	searchURL = "https://efts.sec.gov/LATEST/search-index"

	// Base URL for fetching filing documents.
	archivesURL = "https://www.sec.gov/Archives/edgar/data"
)

// Client is an HTTP client for SEC EDGAR with built-in rate limiting.
type Client struct {
	http    *http.Client
	limiter *ratelimit.Bucket
}

// NewClient creates an EDGAR client with rate limiting.
// SEC allows 10 requests/second; we use 8 to leave headroom.
func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: ratelimit.NewBucket(8, 10),
	}
}

// searchResponse is the JSON structure returned by EDGAR full-text search.
type searchResponse struct {
	Hits struct {
		Hits []struct {
			ID     string `json:"_id"`
			Source struct {
				FileDate     string   `json:"file_date"`
				EntityName   string   `json:"entity_name"`
				Tickers      string   `json:"tickers"`
				DisplayNames []string `json:"display_names"`
			} `json:"_source"`
		} `json:"hits"`
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
	} `json:"hits"`
}

// FetchRecentForm4s retrieves recent Form 4 filing index entries from EDGAR.
// It uses the full-text search API filtered to form type "4".
func (c *Client) FetchRecentForm4s(ctx context.Context, days int) ([]FilingIndex, error) {
	since := time.Now().AddDate(0, 0, -days)
	dateFrom := since.Format("2006-01-02")
	dateTo := time.Now().Format("2006-01-02")

	var allFilings []FilingIndex
	start := 0
	pageSize := 100

	for {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}

		params := url.Values{
			"q":         {`"4"`},
			"forms":     {"4"},
			"dateRange": {"custom"},
			"startdt":   {dateFrom},
			"enddt":     {dateTo},
			"from":      {fmt.Sprintf("%d", start)},
			"size":      {fmt.Sprintf("%d", pageSize)},
		}

		reqURL := "https://efts.sec.gov/LATEST/search-index?" + params.Encode()
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("search request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("EDGAR search returned %d: %s", resp.StatusCode, string(body))
		}

		var result searchResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parse search response: %w", err)
		}

		for _, hit := range result.Hits.Hits {
			filed, _ := time.Parse("2006-01-02", hit.Source.FileDate)
			allFilings = append(allFilings, FilingIndex{
				AccessionNumber: hit.ID,
				FiledAt:         filed,
				URL:             fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s", hit.ID),
			})
		}

		if len(result.Hits.Hits) < pageSize || start+pageSize >= result.Hits.Total.Value {
			break
		}
		start += pageSize
	}

	return allFilings, nil
}

// FetchForm4sByCIK fetches recent Form 4 filings for a specific company CIK
// using the EDGAR submissions API.
func (c *Client) FetchForm4sByCIK(ctx context.Context, cik string, days int) ([]FilingIndex, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	reqURL := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", padCIK(cik))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch submissions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EDGAR returned %d for CIK %s", resp.StatusCode, cik)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// The submissions API nests recent filings under "filings.recent"
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var cikStr string
	if err := json.Unmarshal(raw["cik"], &cikStr); err != nil {
		// cik might be a number
		var cikNum int
		if err2 := json.Unmarshal(raw["cik"], &cikNum); err2 != nil {
			return nil, fmt.Errorf("parse cik: %w", err)
		}
		cikStr = fmt.Sprintf("%d", cikNum)
	}

	var filings struct {
		Recent struct {
			AccessionNumber []string `json:"accessionNumber"`
			FilingDate      []string `json:"filingDate"`
			PrimaryDocument []string `json:"primaryDocument"`
			Form            []string `json:"form"`
		} `json:"recent"`
	}
	if err := json.Unmarshal(raw["filings"], &filings); err != nil {
		return nil, fmt.Errorf("parse filings: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var results []FilingIndex
	for i, form := range filings.Recent.Form {
		if form != "4" {
			continue
		}
		filed, _ := time.Parse("2006-01-02", filings.Recent.FilingDate[i])
		if filed.Before(cutoff) {
			continue
		}
		accession := filings.Recent.AccessionNumber[i]
		results = append(results, FilingIndex{
			AccessionNumber: accession,
			CIK:             cikStr,
			FiledAt:          filed,
			URL: fmt.Sprintf("%s/%s/%s/%s",
				archivesURL,
				padCIK(cikStr),
				dashless(accession),
				filings.Recent.PrimaryDocument[i],
			),
		})
	}

	return results, nil
}

// FetchFilingXML downloads the XML content of a Form 4 filing.
func (c *Client) FetchFilingXML(ctx context.Context, filingURL string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", filingURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch filing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("filing fetch returned %d: %s", resp.StatusCode, filingURL)
	}

	return io.ReadAll(resp.Body)
}

// padCIK zero-pads a CIK to 10 digits as EDGAR requires.
func padCIK(cik string) string {
	for len(cik) < 10 {
		cik = "0" + cik
	}
	return cik
}

// dashless removes dashes from accession numbers for URL construction.
func dashless(accession string) string {
	result := make([]byte, 0, len(accession))
	for i := 0; i < len(accession); i++ {
		if accession[i] != '-' {
			result = append(result, accession[i])
		}
	}
	return string(result)
}
