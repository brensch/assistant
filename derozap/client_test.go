package derozap_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/brensch/assistant/derozap"
)

// TestClientIntegration performs a real integration test with the Dero ZAP system.
// This test will actually log in to the system and fetch real data.
//
// To run this test:
// 1. Set environment variables:
//   - DEROZAP_USERNAME: Your Dero ZAP username/email
//   - DEROZAP_PASSWORD: Your Dero ZAP password
//
// 2. Run: go test -v -run=TestClientIntegration
//
// Note: This test is disabled by default and will be skipped unless you
// explicitly set the environment variables.
func TestClientIntegration(t *testing.T) {
	// Get credentials from environment variables
	username := os.Getenv("DEROZAP_USERNAME")
	password := os.Getenv("DEROZAP_PASSWORD")

	// Skip test if credentials aren't provided
	if username == "" || password == "" {
		t.Skip("Skipping integration test: DEROZAP_USERNAME and DEROZAP_PASSWORD environment variables must be set")
	}

	// Create a new client with the provided credentials
	client, err := derozap.NewClient(username, password)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test logging in
	t.Log("Attempting to log in...")
	err = client.Login()
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	t.Log("Login successful")

	// Set date range for last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	startDateStr := startDate.Format("01/02/2006")
	endDateStr := endDate.Format("01/02/2006")

	t.Logf("Fetching tag reads from %s to %s...", startDateStr, endDateStr)

	// Fetch tag reads for the specified date range
	tagReads, err := client.FetchTagReads(
		derozap.WithDateRange(startDateStr, endDateStr),
		derozap.WithResultsPerPage(50),
	)

	if err != nil {
		t.Fatalf("Failed to fetch tag reads: %v", err)
	}

	// Print summary of results
	t.Logf("Successfully fetched %d tag reads", len(tagReads))

	// Print first few results if available
	if len(tagReads) > 0 {
		t.Log("Sample tag reads:")
		limit := 5
		if len(tagReads) < limit {
			limit = len(tagReads)
		}

		for i := 0; i < limit; i++ {
			t.Logf("  %d. Date: %s, Tag: %s", i+1, tagReads[i].Date, tagReads[i].TagID)
		}
	} else {
		t.Log("No tag reads found for the specified date range")
	}

	// Count unique dates and tags
	dates := make(map[string]bool)
	tags := make(map[string]bool)

	for _, read := range tagReads {
		dates[read.Date] = true
		tags[read.TagID] = true
	}

	t.Logf("Found %d unique dates and %d unique tag IDs", len(dates), len(tags))
}

// Example usage to run the test manually
func Example_clientIntegration() {
	// Set credentials
	username := "your.email@example.com"
	password := "your-password"

	// Create client
	client, err := derozap.NewClient(username, password)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// Login
	err = client.Login()
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		return
	}

	fmt.Println("Successfully logged in")

	// Set date range for last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)
	startDateStr := startDate.Format("01/02/2006")
	endDateStr := endDate.Format("01/02/2006")

	// Fetch tag reads
	tagReads, err := client.FetchTagReads(
		derozap.WithDateRange(startDateStr, endDateStr),
		derozap.WithResultsPerPage(50),
	)

	if err != nil {
		fmt.Printf("Failed to fetch tag reads: %v\n", err)
		return
	}

	fmt.Printf("Successfully fetched %d tag reads\n", len(tagReads))

	// Print first few results
	if len(tagReads) > 0 {
		fmt.Println("First 5 tag reads:")
		limit := 5
		if len(tagReads) < limit {
			limit = len(tagReads)
		}

		for i := 0; i < limit; i++ {
			fmt.Printf("  %d. Date: %s, Tag: %s\n", i+1, tagReads[i].Date, tagReads[i].TagID)
		}
	}
}
