// SPDX-License-Identifier: Apache-2.0
//
// The OpenSearch Contributors require contributions made to
// this file be licensed under the Apache-2.0 license or a
// compatible open source license.
//
// Modifications Copyright OpenSearch Contributors. See
// GitHub history for details.

// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// +build integration

package opensearchutil_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nicolascb/opensearch/v2"
	"github.com/nicolascb/opensearch/v2/opensearchtransport"
	"github.com/nicolascb/opensearch/v2/opensearchutil"
)

func TestBulkIndexerIntegration(t *testing.T) {
	body := `{"body":"Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."}`

	testCases := []struct {
		name                       string
		CompressRequestBodyEnabled bool
	}{
		{
			name:                       "Without body compression",
			CompressRequestBodyEnabled: false,
		},
		{
			name:                       "With body compression",
			CompressRequestBodyEnabled: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("Default", func(t *testing.T) {
				var countSuccessful uint64
				indexName := "test-bulk-integration"

				client, _ := opensearch.NewClient(opensearch.Config{
					CompressRequestBody: tt.CompressRequestBodyEnabled,
					Logger:              &opensearchtransport.ColorLogger{Output: os.Stdout},
				})

				client.Indices.Delete([]string{indexName}, client.Indices.Delete.WithIgnoreUnavailable(true))
				client.Indices.Create(
					indexName,
					client.Indices.Create.WithBody(strings.NewReader(`{"settings": {"number_of_shards": 1, "number_of_replicas": 0, "refresh_interval":"5s"}}`)),
					client.Indices.Create.WithWaitForActiveShards("1"))

				bi, _ := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
					Index:  indexName,
					Client: client,
					// FlushBytes: 3e+6,
				})

				numItems := 100000
				start := time.Now().UTC()

				for i := 1; i <= numItems; i++ {
					err := bi.Add(context.Background(), opensearchutil.BulkIndexerItem{
						Action:     "index",
						DocumentID: strconv.Itoa(i),
						Body:       strings.NewReader(body),
						OnSuccess: func(ctx context.Context, item opensearchutil.BulkIndexerItem, res opensearchutil.BulkIndexerResponseItem) {
							atomic.AddUint64(&countSuccessful, 1)
						},
					})
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
				}

				if err := bi.Close(context.Background()); err != nil {
					t.Errorf("Unexpected error: %s", err)
				}

				stats := bi.Stats()

				if stats.NumAdded != uint64(numItems) {
					t.Errorf("Unexpected NumAdded: want=%d, got=%d", numItems, stats.NumAdded)
				}

				if stats.NumIndexed != uint64(numItems) {
					t.Errorf("Unexpected NumIndexed: want=%d, got=%d", numItems, stats.NumIndexed)
				}

				if stats.NumFailed != 0 {
					t.Errorf("Unexpected NumFailed: want=0, got=%d", stats.NumFailed)
				}

				if countSuccessful != uint64(numItems) {
					t.Errorf("Unexpected countSuccessful: want=%d, got=%d", numItems, countSuccessful)
				}

				fmt.Printf("  Added %d documents to indexer. Succeeded: %d. Failed: %d. Requests: %d. Duration: %s (%.0f docs/sec)\n",
					stats.NumAdded,
					stats.NumFlushed,
					stats.NumFailed,
					stats.NumRequests,
					time.Since(start).Truncate(time.Millisecond),
					1000.0/float64(time.Since(start)/time.Millisecond)*float64(stats.NumFlushed))
			})

			t.Run("Multiple indices", func(t *testing.T) {
				client, _ := opensearch.NewClient(opensearch.Config{
					CompressRequestBody: tt.CompressRequestBodyEnabled,
					Logger:              &opensearchtransport.ColorLogger{Output: os.Stdout},
				})

				bi, _ := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
					Index:  "test-index-a",
					Client: client,
				})

				// Default index
				for i := 1; i <= 10; i++ {
					err := bi.Add(context.Background(), opensearchutil.BulkIndexerItem{
						Action:     "index",
						DocumentID: strconv.Itoa(i),
						Body:       strings.NewReader(body),
					})
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
				}

				// Index 1
				for i := 1; i <= 10; i++ {
					err := bi.Add(context.Background(), opensearchutil.BulkIndexerItem{
						Action: "index",
						Index:  "test-index-b",
						Body:   strings.NewReader(body),
					})
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
				}

				// Index 2
				for i := 1; i <= 10; i++ {
					err := bi.Add(context.Background(), opensearchutil.BulkIndexerItem{
						Action: "index",
						Index:  "test-index-c",
						Body:   strings.NewReader(body),
					})
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
				}

				if err := bi.Close(context.Background()); err != nil {
					t.Errorf("Unexpected error: %s", err)
				}
				stats := bi.Stats()

				expectedIndexed := 10 + 10 + 10
				if stats.NumIndexed != uint64(expectedIndexed) {
					t.Errorf("Unexpected NumIndexed: want=%d, got=%d", expectedIndexed, stats.NumIndexed)
				}

				res, err := client.Indices.Exists([]string{"test-index-a", "test-index-b", "test-index-c"})
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}
				if res.StatusCode != 200 {
					t.Errorf("Expected indices to exist, but got a [%s] response", res.Status())
				}
			})
		})
	}
}
