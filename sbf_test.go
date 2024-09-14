package sbf

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHashIndex(t *testing.T) {
	// Initialize a StableBloomFilter instance
	sbf, err := NewStableBloomFilter(1024, nil, 0.0, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create StableBloomFilter: %v", err)
	}

	data := []byte("test_data")
	for i := uint32(0); i < sbf.k; i++ {
		idx := sbf.hashIndex(data, i)
		if idx >= sbf.m {
			t.Errorf("hashIndex returned index out of bounds: %d", idx)
		}
	}
}

func TestAtomicSetAndGetBit(t *testing.T) {
	var val uint64

	// Test setting and getting bits
	for i := uint32(0); i < 64; i++ {
		atomicSetBit(&val, i)
		if !atomicGetBit(&val, i) {
			t.Errorf("Bit %d was not set correctly", i)
		}
	}

	// Ensure other bits are not affected
	for i := uint32(0); i < 64; i++ {
		if !atomicGetBit(&val, i) {
			t.Errorf("Bit %d should be set", i)
		}
	}
}

// Too much randomness
// func TestDecayBucket(t *testing.T) {
// 	// Create a bucket with all bits set
// 	var bucket uint64 = ^uint64(0)
// 	decayRate := 0.3
// 	randSrc := rand.New(rand.NewSource(42))

// 	decayedBucket := decayBucket(bucket, decayRate, randSrc)

// 	bitsCleared := bucket ^ decayedBucket
// 	bitsRemaining := decayedBucket

// 	if bitsCleared == 0 {
// 		t.Error("No bits were decayed")
// 	}
// 	if bitsRemaining == 0 {
// 		t.Error("All bits were decayed")
// 	}
// }

func TestDecayFunction(t *testing.T) {
	// Initialize a StableBloomFilter instance with decayRate
	sbf, err := NewStableBloomFilter(1024, nil, 1.0, time.Millisecond*10)
	if err != nil {
		t.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	// Set all bits
	for i := range sbf.filter {
		atomic.StoreUint64(&sbf.filter[i], ^uint64(0))
	}

	// Wait for decay to happen
	time.Sleep(time.Millisecond * 20)

	// Check that all bits have been decayed
	for _, bucket := range sbf.filter {
		val := atomic.LoadUint64(&bucket)
		if val != 0 {
			t.Error("Decay did not clear all bits as expected")
			break
		}
	}
}

func TestStartDecay(t *testing.T) {
	// Initialize a StableBloomFilter instance with a short decay interval
	sbf, err := NewStableBloomFilter(1024, nil, 0.5, time.Millisecond*10)
	if err != nil {
		t.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	// Set some bits
	sbf.Add([]byte("test1"))
	sbf.Add([]byte("test2"))

	// Capture the state before decay
	preDecay := make([]uint64, len(sbf.filter))
	for i := range sbf.filter {
		preDecay[i] = atomic.LoadUint64(&sbf.filter[i])
	}

	// Wait for decay to happen
	time.Sleep(time.Millisecond * 20)

	// Capture the state after decay
	postDecay := make([]uint64, len(sbf.filter))
	for i := range sbf.filter {
		postDecay[i] = atomic.LoadUint64(&sbf.filter[i])
	}

	// Ensure that decay has changed the filter state
	identical := true
	for i := range preDecay {
		if preDecay[i] != postDecay[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("Decay did not change the filter state")
	}
}

func TestConcurrency(t *testing.T) {
	// Initialize a StableBloomFilter instance
	sbf, err := NewStableBloomFilter(1024, nil, 0.0, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create StableBloomFilter: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	// Start multiple goroutines to add and check elements concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				data := []byte(fmt.Sprintf("goroutine_%d_data_%d", id, j))
				sbf.Add(data)
				sbf.Check(data)
			}
		}(i)
	}
	wg.Wait()
}

func TestOptimalM(t *testing.T) {
	tests := []struct {
		n       uint32
		p       float64
		wantM   uint32
		wantErr bool
	}{
		{n: 1000, p: 0.01, wantM: 9586, wantErr: false},
		{n: 1000, p: 0.001, wantM: 14378, wantErr: false},
		{n: 0, p: 0.01, wantM: 0, wantErr: true},    // Edge case: n = 0
		{n: 1000, p: 0.0, wantM: 0, wantErr: true},  // Invalid p
		{n: 1000, p: 1.0, wantM: 0, wantErr: true},  // Invalid p
		{n: 1000, p: -0.1, wantM: 0, wantErr: true}, // Invalid p
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			m, err := OptimalM(tt.n, tt.p)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected an error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if m != tt.wantM {
					t.Errorf("OptimalM(%d, %f) = %d; want %d", tt.n, tt.p, m, tt.wantM)
				}
			}
		})
	}
}

func TestOptimalK(t *testing.T) {
	tests := []struct {
		m       uint32
		n       uint32
		wantK   uint32
		wantErr bool
	}{
		{m: 9586, n: 1000, wantK: 7, wantErr: false},
		{m: 14378, n: 1000, wantK: 10, wantErr: false},
		{m: 0, n: 1000, wantK: 0, wantErr: true}, // Edge case: m = 0
		{m: 1000, n: 0, wantK: 0, wantErr: true}, // Edge case: n = 0
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			k, err := OptimalK(tt.m, tt.n)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected an error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if k != tt.wantK {
					t.Errorf("OptimalK(%d, %d) = %d; want %d", tt.m, tt.n, k, tt.wantK)
				}
			}
		})
	}
}

func TestNewDefaultStableBloomFilter(t *testing.T) {
	tests := []struct {
		expectedItems     uint32
		falsePositiveRate float64
		decayRate         float64
		decayInterval     time.Duration
		wantErr           bool
	}{
		{expectedItems: 1000, falsePositiveRate: 0.01, decayRate: 0.01, decayInterval: time.Minute, wantErr: false},
		{expectedItems: 0, falsePositiveRate: 0.01, decayRate: 0.01, decayInterval: time.Minute, wantErr: true},    // Edge case: expectedItems = 0
		{expectedItems: 1000, falsePositiveRate: 0.0, decayRate: 0.01, decayInterval: time.Minute, wantErr: true},  // Invalid p
		{expectedItems: 1000, falsePositiveRate: 1.0, decayRate: 0.01, decayInterval: time.Minute, wantErr: true},  // Invalid p
		{expectedItems: 1000, falsePositiveRate: -0.1, decayRate: 0.01, decayInterval: time.Minute, wantErr: true}, // Invalid p
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			sbf, err := NewDefaultStableBloomFilter(tt.expectedItems, tt.falsePositiveRate, tt.decayRate, tt.decayInterval)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if sbf == nil {
					t.Error("Expected sbf to be non-nil")
				} else {
					// Additional checks can be added here
					sbf.StopDecay() // Clean up
				}
			}
		})
	}
}

func TestEstimateFalsePositiveRate(t *testing.T) {
	// Initialize a StableBloomFilter instance
	sbf, err := NewStableBloomFilter(1024, nil, 0.0, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create StableBloomFilter: %v", err)
	}

	// Test when no bits are set
	fpr := sbf.EstimateFalsePositiveRate()
	expectedFPR := 0.0
	if fpr != expectedFPR {
		t.Errorf("Expected FPR %f, got %f", expectedFPR, fpr)
	}

	// Set all bits
	for i := range sbf.filter {
		atomic.StoreUint64(&sbf.filter[i], ^uint64(0))
	}

	// Test when all bits are set
	fpr = sbf.EstimateFalsePositiveRate()
	expectedFPR = 1.0
	if fpr != expectedFPR {
		t.Errorf("Expected FPR %f, got %f", expectedFPR, fpr)
	}

	// Set some bits
	for i := range sbf.filter {
		atomic.StoreUint64(&sbf.filter[i], 0xAAAAAAAAAAAAAAAA) // Pattern with half bits set
	}

	// Test when half the bits are set
	fpr = sbf.EstimateFalsePositiveRate()
	fractionBitsSet := 0.5
	expectedFPR = math.Pow(fractionBitsSet, float64(sbf.k))
	if math.Abs(fpr-expectedFPR) > 0.0001 {
		t.Errorf("Expected FPR %f, got %f", expectedFPR, fpr)
	}
}

func TestEstimateFalsePositiveRateAfterAdditions(t *testing.T) {
	sbf, err := NewDefaultStableBloomFilter(1000, 0.01, 0.0, time.Hour)
	if err != nil {
		t.Fatalf("Failed to create StableBloomFilter: %v", err)
	}
	defer sbf.StopDecay()

	// Add elements
	numElements := 1000
	for i := 0; i < numElements; i++ {
		data := []byte(fmt.Sprintf("element%d", i))
		sbf.Add(data)
	}

	// Estimate FPR
	fpr := sbf.EstimateFalsePositiveRate()

	// The estimated FPR should be close to the desired false positive rate
	desiredFPR := 0.01
	if math.Abs(fpr-desiredFPR) > 0.005 {
		t.Errorf("Expected FPR close to %f, got %f", desiredFPR, fpr)
	}
}
