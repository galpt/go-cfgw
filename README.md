# go-cfgw

A small, modular, production-minded Go tool to manage Cloudflare Gateway filter lists (port of the original Node.js Cloudflare Gateway Pi-hole Scripts).

> [!TIP]
> This project aims to be a small, dependency-minimal, and thread-safe reimplementation of the original Node scripts. It focuses on reliability (retries with backoff, rate-limit handling, and sensible defaults).

## Table of contents

- [Status](#status)
- [Features](#features)
- [Requirements](#requirements)
- [Build](#build)
- [Usage](#usage)
- [Design notes](#design-notes)
- [Limitations & next steps](#limitations--next-steps)
- [Contributing](#contributing)
- [License](#license)

## Status

- Core features implemented: download lists, normalize/dedupe entries, create Cloudflare Zero Trust lists in chunks, and upsert a Gateway rule that references those lists.
- Production-minded HTTP client with retries, backoff, and Retry-After handling.
- Scheduled GitHub Actions workflow provided to run hourly.

## Features

- Download allowlists and blocklists from configurable sources.
- Sequential-safe downloads to avoid burst-rate issues; small concurrency for speed.
- Robust Cloudflare client handling 429 rate limiting and transient network failures with exponential backoff and jitter.
- Chunked list creation to stay within Cloudflare per-list size limits.
- Modular code structure for easy maintenance and extension.

## Requirements

- Go 1.20+
- A Cloudflare Zero Trust API token with scoped permissions for account-level Gateway Lists and Rules. Provide secrets via environment variables or GitHub Actions secrets.

## Build

These commands work on PowerShell (Windows) and POSIX shells.

```powershell
cd go-cfgw
go mod tidy
go build ./cmd/go-cfgw
# Produces go-cfgw.exe on Windows
```

## Usage

The tool prefers environment variables for configuration. The minimal variables required are:

- `CLOUDFLARE_API_TOKEN` — the API token with appropriate Gateway scopes
- `CLOUDFLARE_ACCOUNT_ID` — your Cloudflare account ID

Optional environment variables are documented inside `cmd/go-cfgw/main.go`.

Example (PowerShell):

```powershell
# Run once
.\go-cfgw.exe

# Or run with env in one line (PowerShell example)
$env:CLOUDFLARE_API_TOKEN = 'xxx'; $env:CLOUDFLARE_ACCOUNT_ID = 'acctid'; .\go-cfgw.exe
```

### GitHub Actions (automatic hourly run)

This repository includes a scheduled workflow (`.github/workflows/go-cfgw-schedule.yml`) that runs every hour. Add `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` as repository secrets.

## Design notes

- The Cloudflare client implements retries with `cenkalti/backoff` and respects `Retry-After` headers on 429 responses.
- Downloads are done sequentially by default to reduce burst load on remote maintainers. The implementation can be tuned via environment variables.
- The project is organized into small packages: `cf` (Cloudflare client), `downloader` (list fetch & normalize), `worker` (orchestration), and `cmd` (CLI entrypoint).

## Limitations & next steps

1. Wirefilter expression generation is kept conservative — test in a staging account before enabling in production.
2. More features from the original project (notifications, some convenience scripts) can be ported as needed.

## Contributing

Contributions welcome. Open issues or PRs. If you'd like additional parity with the original Node scripts (webhooks, more list sources), I can add them.

## License

MIT. See the `LICENSE` file.
