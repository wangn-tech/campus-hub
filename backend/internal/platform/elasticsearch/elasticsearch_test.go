package elasticsearch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wangn-tech/campus-hub/internal/config"
)

func TestSearchBuildsPublicFiltersAndParsesIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/activities/_search" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Fatal("missing query body")
		}
		var query map[string]any
		if err := json.Unmarshal(body, &query); err != nil {
			t.Fatal(err)
		}
		filters := query["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any)
		if len(filters) < 3 {
			t.Fatalf("filters = %#v", filters)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":2},"hits":[{"_source":{"id":"a"}},{"_source":{"id":"b"}}]}}`))
	}))
	defer server.Close()
	client, err := Open(config.ElasticsearchConfig{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	lon, lat := 116.4, 39.9
	status := 3
	result, err := client.Search(context.Background(), "activities", SearchQuery{Keyword: "campus", Status: &status, Longitude: &lon, Latitude: &lat, Distance: "5km", Sort: "distance", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.IDs) != 2 || result.IDs[0] != "a" || result.IDs[1] != "b" {
		t.Fatalf("result = %+v", result)
	}
}
