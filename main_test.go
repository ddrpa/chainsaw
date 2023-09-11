package main

import "testing"

func BenchmarkAppendWithCap(b *testing.B) {
	buffer := make([]string, 0, 50000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buffer = append(buffer, "x")
	}
}

func BenchmarkAppendWithoutCap(b *testing.B) {
	buffer := make([]string, 0)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buffer = append(buffer, "x")
	}
}
