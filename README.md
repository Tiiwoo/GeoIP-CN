# GeoIP-CN

GeoIP-CN merges Chinese IPv4 and IPv6 ranges from multiple sources into a MaxMind DB (MMDB) file. The default database contains `CN` and `PRIVATE` records in `country.iso_code`.

`PRIVATE` is a routing label for the special-use ranges listed in `parse.go`, including private, loopback, link-local, shared, documentation, and multicast ranges. It is not an ISO country code. Consumers must support this label if they use it in routing rules.

## Build and run

Requires Go 1.26 or later. Prepare the IPInfo Lite database with your IPInfo token, then run the tool from the repository directory:

```bash
export IPINFO_TOKEN="your-token"
mkdir -p data
curl --fail --location --silent --show-error \
  --connect-timeout 15 --max-time 120 \
  --retry 2 --retry-max-time 360 --max-filesize 536870912 \
  "https://ipinfo.io/data/ipinfo_lite.mmdb?token=${IPINFO_TOKEN}" \
  --output ./data/ipinfo_lite.mmdb

go build -o geoip .
./geoip -c config.json
```

The default output is `./output/Country.mmdb`. Local paths are relative to the working directory, including when `-c` points to a config elsewhere. Published databases are available from [GitHub Releases](https://github.com/Tiiwoo/GeoIP-CN/releases/latest) and the `release` branch.

## Configuration

See `config.json` for the default configuration. Supported sources:

| Type | Required fields | Optional fields |
| --- | --- | --- |
| `maxmind_mmdb` | `url` | `wantedList` |
| `ipinfo_mmdb` | `url` | `wantedList` |
| `text` | `url`, `name` | `onlyIPType`: `ipv4` or `ipv6` |
| `private` | None | None |

`url` accepts an HTTP(S) URL or a local file path. MMDB sources may specify the countries to retain with `wantedList`; omitting it selects all classifications found in that source. Each requested classification must be present. A source with no matching ranges is an error, even if another source contains the same country.

Text sources accept CIDRs and individual IPs, with `#` or `//` comments. Address-family filtering happens after comment removal and address parsing. IPv4-mapped addresses are treated as IPv4: for example, `::ffff:1.2.3.0/120` becomes `1.2.3.0/24`. Mapped prefixes broader than `/96` are rejected.

`output.file`, `output.dir`, and a nonempty `output.wantedList` are required. Requested output entries must exist and contain ranges. Names are case-insensitive; blank or duplicate names and unknown JSON fields are rejected.

Ranges in the same classification are combined and deduplicated. For overlaps between different classifications, later entries in `output.wantedList` take precedence; the default order makes `PRIVATE` override `CN`.

Downloads use a two-minute timeout per attempt, at most three attempts for transient failures, and a 512 MiB limit per source. Local files have the same size limit. At most four sources are fetched concurrently.

The output is written to a temporary file in the destination directory, synced, closed, and verified before replacing the old file. Write or validation failures preserve the previous database. Replacement is atomic on Unix filesystems. Existing file permissions are preserved; new files are private (`0600`, subject to umask).

## Validation

The regression suite uses local fixtures and HTTP test servers; no IPInfo token is needed:

```bash
go vet ./...
go test -race ./...
```

After generating the default database, run both structural and release-content checks:

```bash
go install github.com/maxmind/mmdbverify@v1.0.0
"$(go env GOPATH)/bin/mmdbverify" -file ./output/Country.mmdb
GEOIP_MMDB_FILE=./output/Country.mmdb go test -v -count=1 -run '^TestReleaseDatabase$' .
```

The release check requires both IPv4 and IPv6 coverage for `CN` and `PRIVATE`, rejects unexpected classifications, and checks representative Chinese, private, and non-Chinese IPs. It is intended for the default database; custom configurations can have different coverage. These checks catch missing data and obvious classification errors, but do not prove every upstream range is geographically correct.

## Automation

Pull requests run formatting, build, vet, and race-enabled tests without publishing. Pushes to `main`, the Thursday schedule, and manual runs on `main` also generate and publish the database after validation. Configure the repository's `IPINFO_TOKEN` secret; publishing uses the automatically provided `GITHUB_TOKEN` with `contents: write` permission scoped to the build job.

Publishing jobs are serialized so they cannot concurrently overwrite the `release` branch. The workflow pins its Actions and `mmdbverify` versions.
