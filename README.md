# go-cfgw

A small, modular, production-minded Go tool to manage Cloudflare Gateway filter lists (port of the original Node.js [Cloudflare Gateway Pi-hole Scripts](https://github.com/mrrfv/cloudflare-gateway-pihole-scripts)).

> [!TIP]
> This project aims to be a small, dependency-minimal, and thread-safe reimplementation of the original Node scripts. It focuses on reliability (retries with backoff, rate-limit handling, and sensible defaults).

## Table of contents

- [Status](#status)
- [Features](#features)
- [Requirements](#requirements)
- [Build](#build)
- [Usage](#usage)
  - [Migration from Node.js version](#migration-from-nodejs-version)
  - [GitHub Actions (automatic hourly run)](#github-actions-automatic-hourly-run)
- [Design notes](#design-notes)
- [Limitations & next steps](#limitations--next-steps)
- [Contributing](#contributing)
- [License](#license)

> [!IMPORTANT]
> **Migrating from the Node.js version?** See [MIGRATION.md](MIGRATION.md) for detailed migration guide and troubleshooting.

## Status

- Core features implemented: download lists, normalize/dedupe entries, create Cloudflare Zero Trust lists in chunks, and upsert a Gateway rule that references those lists.
- Production-minded HTTP client with retries, backoff, and Retry-After handling.
- **Automatic cleanup**: Removes all old CGPS and Go-CFGW artifacts before creating new resources.
- **Proper wirefilter expressions**: Generates correct Cloudflare Gateway wirefilter syntax matching the Node.js implementation.
- **SNI support**: Optional SNI-based filtering with l4 rules (set `BLOCK_BASED_ON_SNI=1`).
- Scheduled GitHub Actions workflow provided to run hourly.

## Features

- **Automatic cleanup**: Deletes all old lists and rules (both CGPS and Go-CFGW) before creating new ones, ensuring idempotent operation.
- **Robust restart handling**: Safe to restart after rate limits or connection failures - cleanup ensures no orphaned resources.
- Download allowlists and blocklists from configurable sources.
- Sequential-safe downloads to avoid burst-rate issues; small concurrency for speed.
- Robust Cloudflare client handling 429 rate limiting and transient network failures with exponential backoff and jitter.
- Chunked list creation to stay within Cloudflare per-list size limits.
- Proper wirefilter expression generation matching the Node.js implementation.
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

### Migration from Node.js version

If you're migrating from the original Node.js [cloudflare-gateway-pihole-scripts](https://github.com/mrrfv/cloudflare-gateway-pihole-scripts):

1. **Automatic cleanup**: go-cfgw will automatically detect and remove all old "CGPS List" and "CGPS Filter Lists" resources on first run.
2. **New naming**: Lists are now named "Go-CFGW Block List - Chunk N" and rules are "Go-CFGW Filter Lists".
3. **Idempotent**: Safe to run multiple times - always cleans up before creating new resources.
4. **No manual cleanup needed**: Unlike the Node.js version which required running delete scripts, go-cfgw handles cleanup automatically.

### GitHub Actions (automatic hourly run)

This repository includes a scheduled workflow (`.github/workflows/go-cfgw-schedule.yml`) that runs every hour. Add `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` as repository secrets.

## Design notes

- **Idempotent operation**: The tool always deletes all old CGPS and Go-CFGW lists/rules before creating new ones. This ensures clean state and safe restarts after failures.
- **Migration-safe**: Automatically removes legacy "CGPS List" and "CGPS Filter Lists" artifacts from the Node.js implementation.
- The Cloudflare client implements retries with `cenkalti/backoff` and respects `Retry-After` headers on 429 responses.
- Downloads are done sequentially by default to reduce burst load on remote maintainers. The implementation can be tuned via environment variables.
- Wirefilter expressions are generated to match the original Node.js implementation: `any(dns.domains[*] in $listID) or ...`
- The project is organized into small packages: `cf` (Cloudflare client), `downloader` (list fetch & normalize), `worker` (orchestration), and `cmd` (CLI entrypoint).

## Limitations & next steps

1. ~~Wirefilter expression generation is kept conservative — test in a staging account before enabling in production.~~ ✅ Fixed: Now properly generates wirefilter expressions matching the Node.js implementation.
2. More features from the original project (notifications, some convenience scripts) can be ported as needed.

## Contributing

Contributions welcome. Open issues or PRs. If you'd like additional parity with the original Node scripts (webhooks, more list sources), I can add them.

## License

MIT. See the `LICENSE` file.
