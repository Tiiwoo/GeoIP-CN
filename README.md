# GeoIP-CN

GeoIP-CN is a tool written in Go for generating MMDB format files that only contain Chinese IP ranges. This tool collects Chinese IP address ranges from multiple data sources, including MaxMind GeoLite2 data and other public IP lists, and merges them to generate a unified MMDB file.

## Features

- Collects Chinese IP address ranges from multiple data sources
- Supports both IPv4 and IPv6 addresses
- Generates standard MaxMind DB format files
- Customizable data sources and output files

### IPv4

- https://github.com/misakaio/chnroutes2/raw/refs/heads/master/chnroutes.mmdb
- https://ipinfo.io/data/free/country.mmdb

### IPv6

- https://raw.githubusercontent.com/gaoyifan/china-operator-ip/ip-lists/china6.txt
