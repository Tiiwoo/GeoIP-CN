# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o geoip .          # build binary
./geoip -c config.json       # run conversion (default config: config.json)
go vet ./...                  # lint
go test -race ./...           # offline regression tests
```

Verify the default release output with:
```bash
go install github.com/maxmind/mmdbverify@v1.0.0
"$(go env GOPATH)/bin/mmdbverify" -file ./output/Country.mmdb
GEOIP_MMDB_FILE=./output/Country.mmdb go test -v -count=1 -run '^TestReleaseDatabase$' .
```

## Architecture

Single-purpose CLI tool that merges Chinese IP ranges from multiple sources into one MMDB file. No plugin system, no interfaces, no registry — just straight-line data pipeline.

**Pipeline:** `main.go` orchestrates: load config → concurrent fetch → parse each source → merge entries → write MMDB.

- **config.go** — Strict JSON decoding and validation (`Config`, `Source`, `Output` structs)
- **fetch.go** — HTTP/local file fetching via `errgroup`; up to four concurrent sources, size limits, request timeouts, and bounded retries
- **parse.go** — Four parsers: `parseMaxmindMMDB`, `parseIPInfoMMDB`, `parseText`, `privateEntry`; plus merge helpers and `wantMap` builder
- **entry.go** — `Entry` type wrapping dual `netipx.IPSetBuilder` (IPv4/IPv6 separate); handles CIDR parsing, comment stripping, prefix merging
- **write.go** — MMDB output using `mmdbwriter`; inserts `{"country":{"iso_code":"XX"}}` records, verifies a temporary file, then replaces the output

**Key design decisions:**
- IPv4 and IPv6 use separate `IPSetBuilder` instances per entry (from `go4.org/netipx`) for correct merging
- Entry names are always uppercased (e.g., "CN", "PRIVATE")
- `config.json` source types: `maxmind_mmdb`, `ipinfo_mmdb`, `text`, `private`
- MMDB writer uses `GeoLite2-Country` database type, record size 28, with reserved networks included
- Empty sources, missing requested classifications, and empty requested output entries are fatal. Never publish a structurally valid database as a substitute for missing data.
- Strip comments and parse addresses before family filtering. Mapped IPv4 prefixes must adjust the prefix length when unmapped; propagate all builder and merge errors.
- Preserve existing output on write/validation failure. Keep temporary output private and on the same filesystem, close readers and writers before rename, preserve existing output permissions, and clean up failed temporary files.
- The default release check requires CN and PRIVATE IPv4/IPv6 coverage and sample lookups. Ordinary tests do not fetch upstream data.

## CI

GitHub Actions (`.github/workflows/build.yml`) checks pull requests without secrets or publishing. Thursday schedules, manual runs on main, and pushes to main also generate and validate data before publishing to the `release` branch and GitHub Releases. Requires the `IPINFO_TOKEN` secret; GitHub supplies `GITHUB_TOKEN`. Publishing jobs are serialized and alone receive `contents: write` permission. Actions are pinned by commit and `mmdbverify` by version; update pins deliberately.
