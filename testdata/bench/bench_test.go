package bench

import "testing"

func BenchmarkSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Do nothing
	}
}
