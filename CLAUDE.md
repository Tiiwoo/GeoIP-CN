# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o geoip .          # build binary
./geoip -c config.json       # run conversion (default config: config.json)
go vet ./...                  # lint
```

No test suite exists. Verify output with:
```bash
go install github.com/maxmind/mmdbverify@latest
mmdbverify -file ./output/Country.mmdb
```

## Architecture

Single-purpose CLI tool that merges Chinese IP ranges from multiple sources into one MMDB file. No plugin system, no interfaces, no registry — just straight-line data pipeline.

**Pipeline:** `main.go` orchestrates: load config → concurrent fetch → parse each source → merge entries → write MMDB.

- **config.go** — JSON config deserialization (`Config`, `Source`, `Output` structs)
- **fetch.go** — Concurrent HTTP/local file fetching via `errgroup`; all sources downloaded in parallel
- **parse.go** — Four parsers: `parseMaxmindMMDB`, `parseIPInfoMMDB`, `parseText`, `privateEntry`; plus merge helpers and `wantMap` builder
- **entry.go** — `Entry` type wrapping dual `netipx.IPSetBuilder` (IPv4/IPv6 separate); handles CIDR parsing, comment stripping, prefix merging
- **write.go** — MMDB output using `mmdbwriter`; iterates wanted entries, inserts `{"country":{"iso_code":"XX"}}` records

**Key design decisions:**
- IPv4 and IPv6 use separate `IPSetBuilder` instances per entry (from `go4.org/netipx`) for correct merging
- Entry names are always uppercased (e.g., "CN", "PRIVATE")
- `config.json` source types: `maxmind_mmdb`, `ipinfo_mmdb`, `text`, `private`
- MMDB writer uses `GeoLite2-Country` database type, record size 28, with reserved networks included

## CI

GitHub Actions (`.github/workflows/build.yml`) runs on Thursday schedule, manual trigger, or push to main. Requires `IPINFO_TOKEN` and `GITHUB_TOKEN` secrets. Output pushed to `release` branch and GitHub Releases.
