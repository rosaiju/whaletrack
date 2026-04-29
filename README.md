# whaletrack

[![CI](https://github.com/rosaiju/whaletrack/actions/workflows/ci.yml/badge.svg)](https://github.com/rosaiju/whaletrack/actions/workflows/ci.yml)

Concurrent SEC EDGAR Form 4 scanner that identifies unusual insider trading activity across all US public companies.

Corporate insiders (CEOs, directors, 10%+ shareholders) must file a [Form 4](https://www.sec.gov/about/forms/form4.pdf) with the SEC within 2 business days of trading their company's stock. This tool scans those filings in bulk and surfaces the interesting ones.

## Demo

```
$ whaletrack scan --type sales --min-value 1000000 --days 30

Scanning SEC EDGAR Form 4 filings (last 30 days)...
[================================] 81 filings processed (9.0s, 5 workers)

Insider Sales > $1M (Last 30 Days)
+--------+--------------------------+-----------------------------+----------+--------+------------+
| Ticker | Insider                  | Title                       | Amount   | Shares | Filed      |
+--------+--------------------------+-----------------------------+----------+--------+------------+
| AAPL   | COOK TIMOTHY D           | Chief Executive Officer     |   $16.0M |    63K | 2026-04-02 |
| AVGO   | Velaga S. Ram            | President, ISG              |   $10.6M |    30K | 2026-04-09 |
| AMZN   | Jassy Andrew R           | President and CEO           |    $7.9M |    31K | 2026-04-17 |
| ABNB   | Gebbia Joseph            | Director                    |    $7.8M |    55K | 2026-04-20 |
| AAPL   | O'BRIEN DEIRDRE          | Senior Vice President       |    $7.7M |    30K | 2026-04-02 |
| ABNB   | Gebbia Joseph            | Director                    |    $7.1M |    57K | 2026-04-06 |
| ABNB   | Blecharczyk Nathan       | Chief Strategy Officer      |    $5.2M |    35K | 2026-04-22 |
| AMZN   | Herrington Douglas J     | CEO Worldwide Amazon Stores |    $5.0M |    20K | 2026-04-14 |
| TSLA   | Wilson-Thompson Kathleen | Director                    |    $2.9M |     8K | 2026-03-30 |
+--------+--------------------------+-----------------------------+----------+--------+------------+

12 results.
```

## Architecture

```
                 ┌─────────────┐
                 │  SEC EDGAR  │
                 │   (HTTPS)   │
                 └──────┬──────┘
                        │
                 ┌──────▼──────┐
                 │ Rate Limiter │  Token bucket (8 req/s, burst 10)
                 │  (ratelimit) │
                 └──────┬──────┘
                        │
          ┌─────────────┼─────────────┐
          ▼             ▼             ▼
   ┌────────────┐┌────────────┐┌────────────┐
   │  Worker 1  ││  Worker 2  ││  Worker N  │  Fan-out: concurrent XML fetch
   │  (fetcher) ││  (fetcher) ││  (fetcher) │
   └─────┬──────┘└─────┬──────┘└─────┬──────┘
         │             │             │
         └─────────────┼─────────────┘
                       ▼
              ┌────────────────┐
              │   Processor    │  Parse XML, filter by criteria
              │   (pipeline)   │
              └───────┬────────┘
                      │
              ┌───────▼────────┐
              │  Deduplicator  │  Remove duplicate filings
              └───────┬────────┘
                      │
            ┌─────────┴─────────┐
            ▼                   ▼
     ┌────────────┐      ┌───────────┐
     │ Table View │      │ JSON File │
     │  (stdout)  │      │  (--out)  │
     └────────────┘      └───────────┘
```

**Key design decisions:**

- **Token bucket rate limiter** — SEC EDGAR allows 10 req/s. We use 8 to leave headroom and avoid IP bans.
- **Worker count (default 10)** — Each HTTP request has ~200-500ms of network latency. At 8 req/s rate limit, 10 workers keep the rate limiter fully saturated (workers spend most of their time waiting on I/O, not on the rate limiter). More workers would just idle; fewer would underutilize the rate limit.
- **Fan-out/fan-in concurrency** — N worker goroutines read from a shared channel. Go's channel semantics ensure each filing is processed by exactly one worker. Results fan back into a single processor.
- **Context propagation** — `context.Context` flows through every goroutine. Ctrl+C triggers graceful shutdown: workers finish their current request, channels drain, and the program exits cleanly.
- **Lenient XML parsing** — SEC filings are inconsistently formatted. The parser skips malformed transactions instead of failing the entire filing.

## Usage

### Scan (one-shot)

```bash
# Find insider purchases over $500K in the last 30 days
whaletrack scan --type purchases --min-value 500000 --days 30

# Find all insider sales, export to JSON
whaletrack scan --type sales --days 7 --out sales.json

# Scan a specific company by CIK
whaletrack scan --cik 1326801 --days 90

# Show all transaction types
whaletrack scan --type all --min-value 1000000
```

### Watch (continuous)

```bash
# Monitor for large purchases every 15 minutes
whaletrack watch --min-value 1000000

# Custom interval
whaletrack watch --interval 5m --type purchases
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `purchases` | Filter: `purchases`, `sales`, or `all` |
| `--min-value` | `100000` | Minimum transaction value (USD) |
| `--days` | `30` | Lookback period |
| `--workers` | `10` | Concurrent fetch goroutines |
| `--cik` | | Filter by company CIK number |
| `--out` | | Export results to JSON file |

## Project Structure

```
internal/
  edgar/
    client.go        HTTP client with rate-limited EDGAR API access
    parser.go        Form 4 XML parsing (handles inconsistent SEC formats)
    parser_test.go   XML parsing tests (sample filings, edge cases, malformed input)
    types.go         Filing, Transaction, Owner, Issuer structs
  pipeline/
    fetcher.go       Fan-out: N goroutines fetch filings concurrently
    processor.go     Parse + filter transactions by type/value
    pipeline.go      Orchestrator: wire stages, track progress, deduplicate
    pipeline_test.go Filter logic and deduplication tests
  output/
    table.go         Terminal table formatting
    json.go          JSON export
  ratelimit/
    bucket.go        Token bucket rate limiter
    bucket_test.go   Rate limiting, burst, cancellation, concurrency tests
cmd/
  scan.go            One-shot scan command
  watch.go           Continuous monitoring command
```

## Tests

14 tests across 3 packages covering the core logic:

```
$ go test ./... -v

=== RUN   TestParseForm4                    # Full XML round-trip with real filing structure
=== RUN   TestParseForm4_WrongRootElement   # Rejects non-Form4 XML
=== RUN   TestParseForm4_EmptyDocument      # Handles filings with no transactions
=== RUN   TestParseForm4_MalformedXML       # Handles garbage input
=== RUN   TestParseFloat                    # Comma-separated numbers, whitespace, empty
=== RUN   TestParseBool                     # SEC uses "1"/"true"/"yes" inconsistently
=== RUN   TestMatchesFilter_Purchases       # Purchase filter + min-value threshold
=== RUN   TestMatchesFilter_Sales           # Sale filter excludes purchases
=== RUN   TestMatchesFilter_All             # Empty type filter passes everything
=== RUN   TestDedupeKey                     # Same insider+ticker+date deduplicates
=== RUN   TestBucket_ImmediateTokens        # Burst capacity consumed instantly
=== RUN   TestBucket_RateLimiting           # Blocks when tokens exhausted
=== RUN   TestBucket_ContextCancellation    # Returns error on cancelled context
=== RUN   TestBucket_ConcurrentAccess       # 50 goroutines, no races

PASS (all packages)
```

## Setup

```bash
git clone https://github.com/rosaiju/whaletrack.git
cd whaletrack

# Build (requires Go 1.22+)
go build -o whaletrack .

# Run tests
go test ./... -v

# Run
./whaletrack scan --type purchases --min-value 500000
```

No API keys needed. SEC EDGAR is free and public. The only requirement is a descriptive User-Agent header (included).

## License

MIT
