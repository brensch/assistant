package derozap

import (
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"
)

func TestFetchTagReadsIntegration(t *testing.T) {
	username := os.Getenv("DEROZAP_USERNAME")
	password := os.Getenv("DEROZAP_PASSWORD")
	if username == "" || password == "" {
		t.Skip("Skipping integration test: set DEROZAP_USERNAME and DEROZAP_PASSWORD")
	}

	// Build a Client without touching the DB layer
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	httpClient := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}
	client := &Client{
		httpClient: httpClient,
		username:   username,
		password:   password,
		// leave dbClient nil and skip table creation entirely
	}

	// Fetch
	tagReads, err := client.FetchTagReads()
	if err != nil {
		t.Fatalf("FetchTagReads failed: %v", err)
	}

	t.Logf("Fetched %d tag reads", len(tagReads))
	if len(tagReads) == 0 {
		t.Skip("no tag reads returned (maybe no data in ZAP for your account)")
	}

	// Print first few for manual inspection
	limit := 5
	if len(tagReads) < limit {
		limit = len(tagReads)
	}
	t.Log("Sample reads:")
	for i := 0; i < limit; i++ {
		t.Logf("  %d: Date=%q, TagID=%q", i+1, tagReads[i].Date, tagReads[i].TagID)
	}
}
