package stopw

import "testing"

func BenchmarkStopw(b *testing.B) {
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Start(i)
		s.Stop(i)
	}
}

func BenchmarkResult(b *testing.B) {
	s := New()
	for i := 0; i < 100; i++ {
		s.Start(i)
		s.Stop(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Result()
	}
}
