package export

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/pteich/elastic-query-export/flags"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/elasticsearch"
)

func TestExportE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		version int
		image   string
	}{
		{version: 7, image: "docker.elastic.co/elasticsearch/elasticsearch:7.17.10"},
		{version: 8, image: "docker.elastic.co/elasticsearch/elasticsearch:8.17.0"},
		{version: 9, image: "docker.elastic.co/elasticsearch/elasticsearch:9.2.3"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Elasticsearch_v%d", tt.version), func(t *testing.T) {
			ctx := context.Background()

			esContainer, err := elasticsearch.Run(ctx, tt.image,
				testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
					ContainerRequest: testcontainers.ContainerRequest{
						Env: map[string]string{
							"discovery.type":         "single-node",
							"xpack.security.enabled": "false",
						},
					},
				}),
			)
			if err != nil {
				t.Fatalf("failed to start container: %s", err)
			}
			defer func() {
				if err := esContainer.Terminate(ctx); err != nil {
					t.Fatalf("failed to terminate container: %s", err)
				}
			}()

			endpoint := esContainer.Settings.Address

			// Seed data
			seedData(t, tt.version, endpoint)

			// Run export
			outFileName := fmt.Sprintf("test_output_v%d.csv", tt.version)
			defer os.Remove(outFileName)

			conf := &flags.Flags{
				ElasticURL:       endpoint,
				ElasticVersion:   tt.version,
				ElasticVerifySSL: false,
				Index:            "test-index",
				Query:            "*",
				OutFormat:        flags.FormatCSV,
				Outfile:          outFileName,
				ScrollSize:       100,
				Timefield:        "@timestamp",
			}

			Run(ctx, conf)

			// Verify output
			verifyOutput(t, outFileName, 3)
		})
	}
}

func seedData(t *testing.T, version int, endpoint string) {
	url := endpoint

	// Index some docs
	for i := 1; i <= 3; i++ {
		doc := fmt.Sprintf(`{"@timestamp": "2023-01-01T00:00:0%dZ", "message": "test message %d", "id": %d}`, i, i, i)
		indexDoc(t, url, "test-index", fmt.Sprintf("%d", i), doc)
	}

	// Refresh index
	refreshIndex(t, url, "test-index")
}

func indexDoc(t *testing.T, url, index, id, body string) {
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/%s/_doc/%s", url, index, id), bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("failed to create request: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to index doc: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		t.Fatalf("failed to index doc, status: %d", resp.StatusCode)
	}
}

func refreshIndex(t *testing.T, url, index string) {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/_refresh", url, index), nil)
	if err != nil {
		t.Fatalf("failed to create refresh request: %s", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to refresh index: %s", err)
	}
	defer resp.Body.Close()
}

func verifyOutput(t *testing.T, filename string, expectedLines int) {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read output file: %s", err)
	}

	lines := bytes.Count(data, []byte("\n"))
	// CSV header + expectedLines
	if lines != expectedLines+1 {
		t.Errorf("expected %d lines in output (including header), got %d", expectedLines+1, lines)
	}
}
