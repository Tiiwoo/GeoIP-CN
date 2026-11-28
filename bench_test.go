package main

import (
	"fmt"
	"testing"

	"github.com/maxmind/mmdbwriter/mmdbtype"
)

func benchmarkRecords(n int, record func(int) mmdbtype.Map) map[string]mmdbtype.Map {
	records := make(map[string]mmdbtype.Map, n)
	for i := 0; i < n; i++ {
		records[fmt.Sprintf("1.%d.%d.0/24", i/256, i%256)] = record(i)
	}
	return records
}

func BenchmarkParseMaxmindMMDB(b *testing.B) {
	data := makeMMDB(b, benchmarkRecords(10000, func(i int) mmdbtype.Map {
		code := "CN"
		if i%2 == 1 {
			code = "US"
		}
		return mmdbtype.Map{"country": mmdbtype.Map{"iso_code": mmdbtype.String(code)}}
	}))
	want := wantMap([]string{"cn"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries, err := parseMaxmindMMDB(data, want)
		if err != nil || len(entries) != 1 {
			b.Fatalf("parse failed: entries=%d err=%v", len(entries), err)
		}
	}
}
