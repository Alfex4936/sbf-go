package sbf

import (
	"errors"
	"math"
	"math/bits"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeebo/xxh3"
)

// Hash64 is a hash function that takes a byte slice and returns a uint64 hash value.
type Hash64 func(data []byte) uint64

// StableBloomFilter represents a Stable Bloom Filter data structure.
//
// It allows approximate membership queries with support for element decay over time.
// The filter supports concurrent access and can be safely used by multiple goroutines.
type StableBloomFilter struct {
	m           uint32       // Size of the filter (number of bits)
	k           uint32       // Number of hash functions
	decayRate   float64      // Probability of decaying bits
	filter      []uint64     // Bit array represented as slice of uint64 for efficiency
	numBuckets  uint32       // Number of buckets (filter size divided by 64)
	decayTicker *time.Ticker // Ticker for decay process
	hashFuncs   []Hash64     // Slice of hash functions
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewStableBloomFilter creates a new Stable Bloom Filter with the specified parameters.
//
// If hashFuncs is empty, it uses default hash functions based on zeebo/xxh3.
//
// Parameters:
//   - m: Size of the filter in bits.
//   - hashFuncs: Slice of hash functions to use. If empty, default hash functions are used.
//   - decayRate: Probability of decaying bits during each decay interval (between 0 and 1).
//   - decayInterval: Time duration between decay operations.
//
// Returns:
//   - A pointer to the StableBloomFilter.
//   - An error if initialization fails.
func NewStableBloomFilter(m uint32, hashFuncs []Hash64, decayRate float64, decayInterval time.Duration) (*StableBloomFilter, error) {
	// If no hash functions are provided, use default hash functions.
	if len(hashFuncs) == 0 {
		defaultK := uint32(7) // Default number of hash functions
		hashFuncs = make([]Hash64, defaultK)
		for i := uint32(0); i < defaultK; i++ {
			hashFuncs[i] = makeHashFunc(uint64(i))
		}
	}
	k := uint32(len(hashFuncs))

	// Ensure m is a multiple of 64 for alignment
	if m%64 != 0 {
		m += 64 - (m % 64)
	}

	numBuckets := m / 64

	sbf := &StableBloomFilter{
		m:           m,
		k:           k,
		decayRate:   decayRate,
		filter:      make([]uint64, numBuckets),
		numBuckets:  numBuckets,
		hashFuncs:   hashFuncs,
		decayTicker: time.NewTicker(decayInterval),
		stopChan:    make(chan struct{}),
	}

	// Start decay process
	sbf.wg.Add(1)
	go sbf.startDecay()

	return sbf, nil
}

// NewDefaultStableBloomFilter creates a new Stable Bloom Filter with optimal settings based on expected items and desired false positive rate.
//
// It uses default hash functions based on zeebo/xxh3.
//
// Parameters:
//   - expectedItems: Estimated number of items to be inserted into the filter.
//   - falsePositiveRate: Desired false positive rate (between 0 and 1).
//   - decayRate: Probability of decaying bits during each decay interval (between 0 and 1). If zero, defaults to 0.01.
//   - decayInterval: Time duration between decay operations. If zero, defaults to 1 minute.
//
// Returns:
//   - A pointer to the StableBloomFilter.
//   - An error if initialization fails.
func NewDefaultStableBloomFilter(expectedItems uint32, falsePositiveRate float64, decayRate float64, decayInterval time.Duration) (*StableBloomFilter, error) {
	// Calculate optimal m and k based on expectedItems and falsePositiveRate
	m, err := OptimalM(expectedItems, falsePositiveRate)
	if err != nil {
		return nil, err
	}
	k, err := OptimalK(m, expectedItems)
	if err != nil {
		return nil, err
	}

	// Generate default hash functions
	hashFuncs := make([]Hash64, k)
	for i := uint32(0); i < k; i++ {
		hashFuncs[i] = makeHashFunc(uint64(i))
	}

	// Assign default decayRate if zero
	if decayRate == 0 {
		decayRate = 0.01
	}

	// Assign default decayInterval if zero
	if decayInterval == 0 {
		decayInterval = time.Minute
	}

	return NewStableBloomFilter(m, hashFuncs, decayRate, decayInterval)
}

// Add inserts an element into the Stable Bloom Filter.
//
// The element is represented as a byte slice.
func (sbf *StableBloomFilter) Add(data []byte) {
	for i := uint32(0); i < sbf.k; i++ {
		idx := sbf.hashIndex(data, i)
		bucketIdx := idx / 64
		bitIdx := idx % 64
		atomicSetBit(&sbf.filter[bucketIdx], bitIdx)
	}
}

// Check tests if an element might be in the Stable Bloom Filter.
//
// Returns true if the element might be in the filter, or false if the element is definitely not in the filter.
func (sbf *StableBloomFilter) Check(data []byte) bool {
	for i := uint32(0); i < sbf.k; i++ {
		idx := sbf.hashIndex(data, i)
		bucketIdx := idx / 64
		bitIdx := idx % 64
		if !atomicGetBit(&sbf.filter[bucketIdx], bitIdx) {
			return false
		}
	}
	return true
}

// StopDecay stops the decay process of the Stable Bloom Filter.
//
// This function should be called when the filter is no longer needed to clean up resources.
func (sbf *StableBloomFilter) StopDecay() {
	sbf.decayTicker.Stop()
	close(sbf.stopChan)
	sbf.wg.Wait()
}

// EstimateFalsePositiveRate estimates the current false positive rate of the Stable Bloom Filter.
//
// The estimation is based on the fraction of bits set in the filter and the number of hash functions.
func (sbf *StableBloomFilter) EstimateFalsePositiveRate() float64 {
	// Calculate the fraction of bits set
	var bitsSet uint32
	for _, bucket := range sbf.filter {
		bitsSet += uint32(bits.OnesCount64(atomic.LoadUint64(&bucket)))
	}
	fractionBitsSet := float64(bitsSet) / float64(sbf.m)

	// Use the standard formula for Bloom filters
	return math.Pow(fractionBitsSet, float64(sbf.k))
}

// OptimalM calculates the optimal filter size (number of bits) for a given number of expected items and desired false positive rate.
//
// Parameters:
//   - n: Expected number of items to be inserted into the filter.
//   - p: Desired false positive rate (between 0 and 1).
//
// Returns:
//   - The optimal filter size in bits.
//   - An error if the input parameters are invalid.
func OptimalM(n uint32, p float64) (uint32, error) {
	if n == 0 {
		return 0, errors.New("expected number of items n must be greater than 0")
	}
	if p <= 0.0 || p >= 1.0 {
		return 0, errors.New("false positive rate p must be between 0 and 1 (exclusive)")
	}
	m := -float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)
	return uint32(math.Ceil(m)), nil
}

// OptimalK calculates the optimal number of hash functions for a given filter size and number of expected items.
//
// Parameters:
//   - m: Size of the filter in bits.
//   - n: Expected number of items to be inserted into the filter.
//
// Returns:
//   - The optimal number of hash functions.
//   - An error if the input parameters are invalid.
func OptimalK(m uint32, n uint32) (uint32, error) {
	if n == 0 {
		return 0, errors.New("expected number of items n must be greater than 0")
	}
	if m == 0 {
		return 0, errors.New("filter size m must be greater than 0")
	}
	k := (float64(m) / float64(n)) * math.Ln2
	return uint32(math.Round(k)), nil
}

// hashIndex computes the hash index for the i-th hash function.
func (sbf *StableBloomFilter) hashIndex(data []byte, i uint32) uint32 {
	sum := sbf.hashFuncs[i](data)
	return uint32(sum % uint64(sbf.m))
}

// startDecay periodically decays the filter.
func (sbf *StableBloomFilter) startDecay() {
	defer sbf.wg.Done()
	for {
		select {
		case <-sbf.decayTicker.C:
			sbf.decay()
		case <-sbf.stopChan:
			return
		}
	}
}

// decay unsets bits randomly based on decayRate.
func (sbf *StableBloomFilter) decay() {
	numCPU := runtime.NumCPU()
	var wg sync.WaitGroup
	chunkSize := int(sbf.numBuckets) / numCPU
	if chunkSize == 0 {
		chunkSize = int(sbf.numBuckets)
	}

	for i := 0; i < numCPU; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == numCPU-1 || end > int(sbf.numBuckets) {
			end = int(sbf.numBuckets)
		}

		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			randSrc := rand.New(rand.NewSource(time.Now().UnixNano() + int64(start)))
			decayRate := sbf.decayRate
			for j := start; j < end; j++ {
				oldVal := atomic.LoadUint64(&sbf.filter[j])
				newVal := decayBucket(oldVal, decayRate, randSrc)
				atomic.StoreUint64(&sbf.filter[j], newVal)
			}
		}(start, end)
	}
	wg.Wait()
}

// atomicSetBit sets a bit atomically.
func atomicSetBit(addr *uint64, n uint32) {
	mask := uint64(1) << n
	*addr |= mask // go1.23 >= OrUint64
}

// atomicGetBit gets a bit atomically.
func atomicGetBit(addr *uint64, n uint32) bool {
	val := atomic.LoadUint64(addr)
	return (val & (uint64(1) << n)) != 0
}

// decayBucket decays bits in a bucket based on the decay rate.
func decayBucket(bucket uint64, decayRate float64, randSrc *rand.Rand) uint64 {
	if bucket == 0 {
		return 0
	}

	for bucket != 0 {
		bitPos := bits.TrailingZeros64(bucket)
		if randSrc.Float64() < decayRate {
			bucket &^= 1 << bitPos
		}
		bucket &= bucket - 1 // Clear the least significant bit set
	}
	return bucket
}

// makeHashFunc returns a Hash64 function using xxh3 with a given seed.
func makeHashFunc(seed uint64) Hash64 {
	return func(data []byte) uint64 {
		return xxh3.HashSeed(data, seed)
	}
}
