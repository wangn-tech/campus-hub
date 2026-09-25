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

func TestSwitchAliasUsesElasticsearchCompatibleRemoveAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/_aliases" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Actions []map[string]map[string]any `json:"actions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		remove := body.Actions[0]["remove"]
		if remove["must_exist"] != false {
			t.Fatalf("remove action = %#v, want must_exist=false", remove)
		}
		if _, exists := remove["ignore_unavailable"]; exists {
			t.Fatalf("remove action must not contain ignore_unavailable: %#v", remove)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = w.Write([]byte(`{"acknowledged":true}`))
	}))
	defer server.Close()
	client, err := Open(config.ElasticsearchConfig{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	if err := client.SwitchAlias(context.Background(), "activities", "activities_v2"); err != nil {
		t.Fatal(err)
	}
}

func TestIndexUsesExternalGTEForCompensationReplay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("version_type"); got != "external_gte" {
			t.Fatalf("version_type = %q, want external_gte", got)
		}
		if got := r.URL.Query().Get("version"); got != "7" {
			t.Fatalf("version = %q, want 7", got)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_, _ = w.Write([]byte(`{"result":"updated"}`))
	}))
	defer server.Close()
	client, err := Open(config.ElasticsearchConfig{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	if err := client.Index(context.Background(), "activities", "id", 7, map[string]string{"id": "id"}); err != nil {
		t.Fatal(err)
	}
}
