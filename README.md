# whaletrack

Concurrent SEC EDGAR Form 4 scanner that identifies unusual insider trading activity across all US public companies.

Corporate insiders (CEOs, directors, 10%+ shareholders) must file a [Form 4](https://www.sec.gov/about/forms/form4.pdf) with the SEC within 2 business days of trading their company's stock. This tool scans those filings in bulk and surfaces the interesting ones.

## Demo

```
$ whaletrack scan --type purchases --min-value 500000 --days 30

Scanning SEC EDGAR Form 4 filings (last 30 days)...
[================================] 4,218 filings processed (8.3s, 20 workers)

Insider Purchases > $500K (Last 30 Days)
+--------+----------------------+-----------+---------+--------+------------+
| Ticker | Insider              | Title     | Amount  | Shares | Filed      |
+--------+----------------------+-----------+---------+--------+------------+
| KKR    | Kravis Henry R       | Director  | $21.4M  | 210K   | 2026-02-28 |
| RDDT   | Farrell Sarah        | Director  | $8.9M   | 63K    | 2026-03-22 |
| BMI    | Bockhorst Kenneth W  | CEO       | $762K   | 6K     | 2026-04-15 |
+--------+----------------------+-----------+---------+--------+------------+

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
| `--workers` | `20` | Concurrent fetch goroutines |
| `--cik` | | Filter by company CIK number |
| `--out` | | Export results to JSON file |

## Project Structure

```
internal/
  edgar/
    client.go      HTTP client with rate-limited EDGAR API access
    parser.go      Form 4 XML parsing (handles inconsistent SEC formats)
    types.go       Filing, Transaction, Owner, Issuer structs
  pipeline/
    fetcher.go     Fan-out: N goroutines fetch filings concurrently
    processor.go   Parse + filter transactions by type/value
    pipeline.go    Orchestrator: wire stages, track progress, deduplicate
  output/
    table.go       Terminal table formatting
    json.go        JSON export
  ratelimit/
    bucket.go      Token bucket rate limiter
cmd/
  scan.go          One-shot scan command
  watch.go         Continuous monitoring command
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
