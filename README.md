# Stable Bloom Filter (SBF) for Go

A **Stable Bloom Filter (SBF)** implementation in Go, providing an approximate membership data structure with support for element decay over time.

It allows you to efficiently test whether an element is likely present in a set, with a configurable false positive rate, and automatically forgets old elements based on a decay mechanism.

## Features

- **Approximate Membership Testing**: Quickly check if an element is likely in the set.
- **Element Decay**: Automatically removes old elements over time to prevent filter saturation.
- **Concurrent Access**: Safe for use by multiple goroutines simultaneously.
- **Customizable Parameters**: Configure false positive rate, decay rate, and decay interval.
- **Optimized for Performance**: Efficient memory usage and fast operations.
- **Scalability**: Designed to handle large data sets and high-throughput applications.

## Table of Contents

- [Stable Bloom Filter (SBF) for Go](#stable-bloom-filter-sbf-for-go)
  - [Features](#features)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
  - [Quick Start](#quick-start)
  - [Usage Examples](#usage-examples)
    - [Detecting Duplicates Among Users](#detecting-duplicates-among-users)
  - [When to Use](#when-to-use)
  - [When Not to Use](#when-not-to-use)
  - [Parameters Explanation](#parameters-explanation)
  - [Scalability](#scalability)
    - [Concurrent Access](#concurrent-access)
    - [Memory Efficiency](#memory-efficiency)
    - [Horizontal Scaling](#horizontal-scaling)
    - [Example: Scaling with Multiple Filters](#example-scaling-with-multiple-filters)
    - [Considerations](#considerations)
  - [Performance Considerations](#performance-considerations)
  - [Limitations](#limitations)
  - [License](#license)

## Installation

```bash
go get github.com/Alfex4936/sbf-go
```

## Quick Start

Here's how to create a new Stable Bloom Filter and use it to add and check elements:

```go
package main

import (
    "fmt"
    "github.com/Alfex4936/sbf-go"
    "time"
)

func main() {
    // Parameters for the Stable Bloom Filter
    expectedItems := uint32(1_000_000) // Expected number of items
    falsePositiveRate := 0.01          // Desired false positive rate (1%)

    // Create a Stable Bloom Filter with default decay settings
    sbfInstance, err := sbf.NewDefaultStableBloomFilter(expectedItems, falsePositiveRate, 0, 0)
    if err != nil {
        panic(err)
    }
    defer sbfInstance.StopDecay() // Ensure resources are cleaned up

    // Add an element
    element := []byte("example_element")
    sbfInstance.Add(element)

    // Check if the element is in the filter
    if sbfInstance.Check(element) {
        fmt.Println("Element is probably in the set.")
    } else {
        fmt.Println("Element is definitely not in the set.")
    }

    // Wait for some time to let decay happen
    time.Sleep(2 * time.Minute)

    // Check again after decay
    if sbfInstance.Check(element) {
        fmt.Println("Element is probably still in the set.")
    } else {
        fmt.Println("Element has likely decayed from the set.")
    }
}
```

## Usage Examples

### Detecting Duplicates Among Users

In this example, we'll use the Stable Bloom Filter to detect duplicates among a stream of user registrations. This can be useful in preventing duplicate entries, replay attacks, or filtering repeated events.

```go
package main

import (
    "fmt"
    "math/rand"
    "time"

    "github.com/Alfex4936/sbf-go"
)

func main() {
    // Parameters for the Stable Bloom Filter
    expectedItems := uint32(1_000_000) // Expected number of items
    falsePositiveRate := 0.01          // Desired false positive rate (1%)

    // Create a Stable Bloom Filter with default decay settings
    sbfInstance, err := sbf.NewDefaultStableBloomFilter(expectedItems, falsePositiveRate, 0, 0)
    if err != nil {
        panic(err)
    }
    defer sbfInstance.StopDecay() // Ensure resources are cleaned up

    // Seed the random number generator
    rand.Seed(time.Now().UnixNano())

    // Simulate a stream of usernames with potential duplicates
    totalUsers := 1_000_000
    maxUserID := totalUsers / 2 // Adjust to increase the chance of duplicates
    duplicateCount := 0

    for i := 0; i < totalUsers; i++ {
        // Generate a username (e.g., "user_123456")
        userID := rand.Intn(maxUserID)
        username := fmt.Sprintf("user_%d", userID)

        // Check if the username might have been seen before
        if sbfInstance.Check([]byte(username)) {
            // Likely a duplicate
            duplicateCount++
        } else {
            // New username, add it to the filter
            sbfInstance.Add([]byte(username))
        }
    }

    fmt.Printf("Total users processed: %d\n", totalUsers)
    fmt.Printf("Potential duplicates detected: %d\n", duplicateCount)

    // Estimate the false positive rate after processing
    estimatedFPR := sbfInstance.EstimateFalsePositiveRate()
    fmt.Printf("Estimated false positive rate: %.6f\n", estimatedFPR)
}
```

**Output:**

```
Total users processed: 1000000
Potential duplicates detected: 567435
Estimated false positive rate: 0.000107
```

**Explanation:**

- We simulate one million user registrations, with user IDs ranging from 0 to 499,999. This means duplicates are likely, as we have more registrations than unique user IDs.
- We use the SBF to check for duplicates:
  - If `Check` returns `true`, we increment the `duplicateCount`.
  - If `Check` returns `false`, we add the username to the filter.
- The high number of duplicates detected is expected due to the limited range of user IDs, not because of false positives.
- The estimated false positive rate is very low (`0.0107%`), indicating that almost all duplicates detected are actual duplicates.

## When to Use

- **High Throughput Systems**: Applications that require fast insertion and query times with minimal memory overhead.
- **Streaming Data**: Scenarios where data is continuously flowing, and old data becomes less relevant over time.
- **Duplicate Detection**: Identifying duplicate events or entries without storing all elements.
- **Cache Expiration**: Probabilistically determining if an item is still fresh or should be re-fetched.
- **Approximate Membership Testing**: When exact membership testing is less critical than speed and memory usage.

## When Not to Use

- **Exact Membership Required**: Applications that cannot tolerate false positives or require exact deletions of elements.
- **Small Data Sets**: When the data set is small enough to be stored and managed with precise data structures.
- **Sensitive Data**: Scenarios where the cost of a false positive is too high (e.g., financial transactions, critical security checks).
- **Complex Deletion Requirements**: If you need to delete specific elements immediately, a Stable Bloom Filter is not suitable.

## Parameters Explanation

- **`expectedItems`**: The estimated number of unique items you expect to store in the filter. This helps calculate the optimal size of the filter.
- **`falsePositiveRate`**: The desired probability of false positives. Lowering this value reduces false positives but increases memory usage.
- **`decayRate`**: The probability that each bit in the filter will decay (be unset) during each decay interval. Default is `0.01` (1%).
- **`decayInterval`**: The time duration between each decay operation. Default is `1 * time.Minute`.

**Choosing `decayRate` and `decayInterval`:**

- **Element Retention Time**: If you want elements to persist longer in the filter, decrease the `decayRate` or increase the `decayInterval`.
- **High Insertion Rate**: For applications with high insertion rates, you may need a higher `decayRate` or shorter `decayInterval` to prevent the filter from becoming saturated.

## Scalability

The Stable Bloom Filter is designed to be scalable and can handle large data sets and high-throughput applications efficiently. Here's how:

### Concurrent Access

- **Thread-Safe Operations**: The SBF implementation uses atomic operations, making it safe for concurrent use by multiple goroutines without additional locking mechanisms.
- **High Throughput**: Insertion (`Add`) and query (`Check`) operations are fast and have constant time complexity `O(k)`, where `k` is the number of hash functions. This allows the filter to handle a high rate of operations per second.

### Memory Efficiency

- **Low Memory Footprint**: The SBF is space-efficient, requiring minimal memory to represent large sets. Memory usage is directly related to the desired false positive rate and the expected number of items.
- **Configurable Parameters**: Adjusting the `falsePositiveRate` and `expectedItems` allows you to scale the filter to match your application's memory constraints and performance requirements.

### Horizontal Scaling

- **Sharding Filters**: For extremely large data sets or to distribute load, you can partition your data and use multiple SBF instances (shards). Each shard handles a subset of the data, allowing the system to scale horizontally.
- **Distributed Systems**: In distributed environments, you can deploy SBF instances across multiple nodes, ensuring that each node maintains its own filter or shares filters through a coordination mechanism.

### Example: Scaling with Multiple Filters

```go
// Number of shards (e.g., based on the number of CPUs or nodes)
numShards := 10
sbfShards := make([]*sbf.StableBloomFilter, numShards)

// Initialize each shard
for i := 0; i < numShards; i++ {
    sbfInstance, err := sbf.NewDefaultStableBloomFilter(expectedItems/uint32(numShards), falsePositiveRate, 0, 0)
    if err != nil {
        panic(err)
    }
    sbfShards[i] = sbfInstance
    defer sbfInstance.StopDecay()
}

// Function to determine which shard to use (e.g., based on hash of the element)
func getShardIndex(element []byte) int {
    hashValue := someHashFunction(element)
    return int(hashValue % uint32(numShards))
}

// Adding and checking elements
element := []byte("example_element")
shardIndex := getShardIndex(element)
sbfShards[shardIndex].Add(element)

if sbfShards[shardIndex].Check(element) {
    fmt.Println("Element is probably in the set.")
}
```

**Explanation:**

- **Sharding Logic**: Elements are distributed among shards based on a hash function. This reduces the load on individual filters and allows the system to handle more data and higher throughput.
- **Scalability**: By adding more shards, you can scale horizontally to accommodate growing data volumes or increased performance demands.

### Considerations

- **Consistent Hashing**: Use consistent hashing to minimize data redistribution when adding or removing shards.
- **Synchronization**: In some cases, you might need to synchronize filters or handle cross-shard queries, which can add complexity.
- **Monitoring and Balancing**: Monitor the load on each shard to ensure even distribution and adjust the sharding strategy if necessary.

## Performance Considerations

- **Memory Efficiency**: Bloom filters are space-efficient, requiring minimal memory to represent large sets.
- **Fast Operations**: Both insertion (`Add`) and query (`Check`) operations are fast and have constant time complexity `O(k)`, where `k` is the number of hash functions.
- **Concurrency**: The implementation is safe for concurrent use by multiple goroutines without additional locking mechanisms.
- **Decay Overhead**: The decay process runs in a separate goroutine. The overhead is minimal but should be considered in resource-constrained environments.

## Limitations

- **No Deletion of Specific Elements**: You cannot remove specific elements from the filter. Elements decay over time based on the decay parameters.
- **False Positives**: The filter can return false positives (i.e., it may indicate that an element is present when it's not). The false positive rate is configurable but cannot be entirely eliminated.
- **Not Suitable for Counting**: If you need to count occurrences of elements, consider using a Counting Bloom Filter instead.
- **Sharding Complexity**: While sharding allows horizontal scaling, it introduces additional complexity in managing multiple filters and ensuring consistent hashing.

## License

This project is licensed under the MIT License.
