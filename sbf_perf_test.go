package sbf

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestSBFWithMillionUsers(t *testing.T) {
	expectedItems := uint32(1_000_000)
	falsePositiveRate := 0.01 // Desired false positive rate (1%)
	decayRate := 0.0          // No decay for this test
	decayInterval := time.Hour

	sbf, err := NewDefaultStableBloomFilter(expectedItems, falsePositiveRate, decayRate, decayInterval)
	if err != nil {
		t.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	// Output filter parameters
	t.Logf("Filter parameters: m=%d bits, k=%d hash functions", sbf.m, sbf.k)

	// Estimate memory used by the filter
	filterMemoryBytes := uint64(sbf.m) / 8 // Total bits divided by 8 to get bytes
	t.Logf("Approximate memory used by the filter: %d bytes (%.2f MB)", filterMemoryBytes, float64(filterMemoryBytes)/(1024*1024))

	// Generate and add 1 million random usernames to the filter
	for i := uint32(0); i < expectedItems; i++ {
		username := []byte(fmt.Sprintf("user_%d_%d", i, rand.Int()))
		sbf.Add(username)
	}

	// Measure observed false positive rate
	numTests := 100_000
	falsePositives := 0
	for i := 0; i < numTests; i++ {
		username := []byte(fmt.Sprintf("non_user_%d_%d", i, rand.Int()))
		if sbf.Check(username) {
			falsePositives++
		}
	}

	observedFPR := float64(falsePositives) / float64(numTests)
	t.Logf("Observed false positive rate: %.6f", observedFPR)

	// Output the estimated false positive rate from the filter
	estimatedFPR := sbf.EstimateFalsePositiveRate()
	t.Logf("Estimated false positive rate from filter: %.6f", estimatedFPR)
}

func BenchmarkAddSolo(b *testing.B) {
	sbf, err := NewDefaultStableBloomFilter(1_000_000, 0.01, 0.0, time.Hour)
	if err != nil {
		b.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	data := []byte("benchmark_data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sbf.Add(data)
	}
}

func BenchmarkCheckSolo(b *testing.B) {
	sbf, err := NewDefaultStableBloomFilter(1_000_000, 0.01, 0.0, time.Hour)
	if err != nil {
		b.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	data := []byte("benchmark_data")
	sbf.Add(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sbf.Check(data)
	}
}

func BenchmarkAddConcurrent(b *testing.B) {
	sbf, err := NewDefaultStableBloomFilter(1_000_000, 0.01, 0.0, time.Hour)
	if err != nil {
		b.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	numGoroutines := 100
	var wg sync.WaitGroup

	b.ResetTimer()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("benchmark_data_%d", id))
			for j := 0; j < b.N/numGoroutines; j++ {
				sbf.Add(data)
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkCheckConcurrent(b *testing.B) {
	sbf, err := NewDefaultStableBloomFilter(1_000_000, 0.01, 0.0, time.Hour)
	if err != nil {
		b.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	numGoroutines := 100
	var wg sync.WaitGroup

	// Prepopulate the filter
	for i := 0; i < numGoroutines; i++ {
		data := []byte(fmt.Sprintf("benchmark_data_%d", i))
		sbf.Add(data)
	}

	b.ResetTimer()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("benchmark_data_%d", id))
			for j := 0; j < b.N/numGoroutines; j++ {
				sbf.Check(data)
			}
		}(i)
	}
	wg.Wait()
}
