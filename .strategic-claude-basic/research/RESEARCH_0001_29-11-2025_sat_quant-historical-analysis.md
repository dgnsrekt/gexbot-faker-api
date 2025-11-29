---
date: 2025-11-29T15:50:30-06:00
git_commit: 46035d6a5b7880fe7ab29ea9c5770d5dbd253ea6
branch: main
repository: gexbot-faker-api
topic: "Quant-Historical Repository Analysis - Gexbot API Client"
tags: [research, gexbot, api-client, historical-data, options-data, greeks]
status: complete
last_updated: 2025-11-29
---

# Research: Quant-Historical Repository Analysis

**Date**: 2025-11-29T15:50:30-06:00
**Git Commit**: 46035d6a5b7880fe7ab29ea9c5770d5dbd253ea6
**Branch**: main
**Repository**: gexbot-faker-api

## Research Question

Analyze the quant-historical repository to understand how it works and document the Gexbot API structure for building a faker API.

## Summary

The **quant-historical** repository is a Python client for the Gexbot API that retrieves signed download URLs for historical options/derivatives data. The script queries the `/v2/hist/` endpoint with various ticker/package/category combinations and returns JSON responses containing pre-signed URLs for data file downloads.

Key findings:
- **API Base URL**: `https://api.gex.bot`
- **Endpoint Pattern**: `/v2/hist/{ticker}/{package}/{category}/{date}`
- **Authentication**: Basic auth via `Authorization: Basic {API_KEY}` header
- **Data Types**: GEX (Gamma Exposure), Delta, Gamma, Vanna, Charm - at various levels (zero, one, full)

## Detailed Findings

### API Structure

The Gexbot API follows a RESTful pattern for historical data:

```
GET /v2/hist/{ticker}/{package}/{category}/{date}
```

**Path Parameters:**
| Parameter | Description | Examples |
|-----------|-------------|----------|
| `ticker` | Stock/index symbol | SPX, NDX, SPY, QQQ, AAPL, TSLA, VIX |
| `package` | Data grouping type | state, classic, orderflow |
| `category` | Specific data category | gex_full, delta_zero, gamma_one |
| `date` | Query date | YYYY-MM-DD format |

### Supported Tickers

The repository defines 41 supported tickers across different asset classes:

**Major Indices:**
- SPX, ES_SPX (S&P 500 and futures)
- NDX, NQ_NDX (NASDAQ and futures)
- RUT (Russell 2000)
- VIX (Volatility Index)

**Popular ETFs:**
- SPY, QQQ, IWM (index trackers)
- TQQQ, UVXY (leveraged/volatility)
- TLT, GLD, SLV, USO (bonds/commodities)
- IBIT, HYG (crypto/high yield)

**Mega-cap Stocks:**
- AAPL, MSFT, AMZN, NVDA, META, GOOG/GOOGL, NFLX, TSLA

**High-volatility Names:**
- MSTR, COIN, GME, HOOD, PLTR, IONQ, CRWV, RDDT

### Data Packages

**1. State Package** - Real-time state calculations
- `gex_full` - Full Gamma Exposure data
- `gex_zero` - Zero GEX level
- `gex_one` - GEX at level 1
- `delta_zero`, `delta_one` - Delta exposure levels
- `gamma_zero`, `gamma_one` - Gamma exposure levels
- `vanna_zero`, `vanna_one` - Vanna (delta sensitivity to volatility)
- `charm_zero`, `charm_one` - Charm (delta decay over time)

**2. Classic Package** - Traditional GEX data
- `gex_full` - Full Gamma Exposure

**3. Orderflow Package** - Order flow analysis
- `orderflow` - Order flow metrics

### Authentication Flow

```python
session.headers["Authorization"] = f"Basic {API_KEY}"
session.headers["Accept"] = "application/json"
session.params = {"noredirect": ""}  # Returns JSON with URL instead of 302 redirect
```

The API supports two modes:
1. **Redirect mode** (default): Returns HTTP 302 redirect to signed URL
2. **No-redirect mode**: Returns JSON with `url` field containing signed download URL

### Response Format

Successful response (JSON):
```json
{
  "url": "https://storage.gexbot.com/path/to/data.parquet?signature=..."
}
```

The URL is a pre-signed URL for downloading the actual data file (likely Parquet format).

### Code Architecture

The main.py script follows a simple pattern:

1. **Configuration** (lines 6-108): User selects tickers and categories by uncommenting lists
2. **Combination Generator** (lines 111-132): `generate_combinations()` creates all ticker/package/category permutations
3. **Fetcher** (lines 135-203): `fetch_history_url()` iterates through combinations and makes API calls

## Code References

- `/home/dgnsrekt/Development/GEXBOT_RESEARCH/quant-historical/main.py:11` - API key configuration
- `/home/dgnsrekt/Development/GEXBOT_RESEARCH/quant-historical/main.py:14` - Base URL definition
- `/home/dgnsrekt/Development/GEXBOT_RESEARCH/quant-historical/main.py:22-71` - Supported tickers list
- `/home/dgnsrekt/Development/GEXBOT_RESEARCH/quant-historical/main.py:76-106` - Category definitions by package
- `/home/dgnsrekt/Development/GEXBOT_RESEARCH/quant-historical/main.py:111-132` - Combination generator function
- `/home/dgnsrekt/Development/GEXBOT_RESEARCH/quant-historical/main.py:170` - API endpoint construction pattern

## Architecture Insights

### For Building a Faker API

To create a mock/faker API that mimics the Gexbot API, you would need:

1. **Endpoint Structure**:
   ```
   GET /v2/hist/{ticker}/{package}/{category}/{date}
   ```

2. **Authentication Middleware**:
   - Accept Basic auth header
   - Validate API key format

3. **Response Types**:
   - Support `noredirect` query param
   - Return JSON with signed URL or redirect

4. **Data Generation**:
   - Generate fake signed URLs
   - Optionally serve mock Parquet/data files

5. **Validation**:
   - Validate ticker is in supported list
   - Validate package/category combinations
   - Validate date format (YYYY-MM-DD)

### Valid Package/Category Combinations

| Package | Valid Categories |
|---------|------------------|
| state | gex_full, gex_zero, gex_one, delta_zero, delta_one, gamma_zero, gamma_one, vanna_zero, vanna_one, charm_zero, charm_one |
| classic | gex_full |
| orderflow | orderflow |

## Related Research

No Architecture Decision Records found in `.strategic-claude-basic/decisions/`

## Open Questions

1. **Data file format**: What is the structure of the downloaded files (Parquet schema)?
2. **Rate limiting**: Does the API have rate limits that should be mocked?
3. **Error responses**: What are the exact error response formats for 4xx/5xx errors?
4. **Date range**: What date ranges are valid for historical queries?
5. **Additional endpoints**: Are there other API endpoints beyond `/v2/hist/`?
