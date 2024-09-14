package main

import (
	"fmt"
	"math/rand"

	"github.com/Alfex4936/sbf-go"
)

const USERS = 10_000_000 // 10 millions users

func main() {
	// Parameters for the Stable Bloom Filter
	expectedItems := uint32(USERS) // Expected number of items
	falsePositiveRate := 0.01      // Desired false positive rate (1%)

	// Create a Stable Bloom Filter with default decay settings
	sbfInstance, err := sbf.NewDefaultStableBloomFilter(expectedItems, falsePositiveRate, 0, 0)
	if err != nil {
		panic(err)
	}
	defer sbfInstance.StopDecay() // Ensure resources are cleaned up

	// Simulate a stream of usernames with potential duplicates
	totalUsers := USERS
	maxUserID := totalUsers / 2 // to increase the chance of duplicates

	actualDuplicates := 0
	falsePositives := 0
	usernamesSeen := make(map[string]bool)

	for i := 0; i < totalUsers; i++ {
		// Generate a username (e.g., "user_123456")
		userID := rand.Intn(maxUserID)
		username := fmt.Sprintf("user_%d", userID)

		// Check if the username might have been seen before
		if sbfInstance.Check([]byte(username)) {
			if usernamesSeen[username] {
				actualDuplicates++
			} else {
				falsePositives++
			}
		} else {
			// New username, add it to the filter
			sbfInstance.Add([]byte(username))
		}
		usernamesSeen[username] = true
	}

	fmt.Printf("Total users processed: %d\n", totalUsers)
	fmt.Printf("Actual duplicates detected: %d\n", actualDuplicates)
	fmt.Printf("False positives detected: %d\n", falsePositives)

	// Estimate the false positive rate after processing
	estimatedFPR := sbfInstance.EstimateFalsePositiveRate()
	fmt.Printf("Estimated false positive rate: %.6f\n", estimatedFPR)
}

/*
# Example output:
Total users processed: 1000000
Actual duplicates detected: 567779
False positives detected: 3
Estimated false positive rate: 0.000107

# Percentage of Incorrect Detections

False-Positives/Total Duplicates Detected * 100 %

≈ 14/567_435 * 100% ≈ 0.0025%

So this example shows SBF is detecting duplicates usernames with a 0.0025% false positive rate, which is a reasonable performance for a bloom filter.

Note: This is a simplified example, and in a real-world scenario, you might want to consider additional factors such as memory usage, performance optimization, and error handling.
*/
