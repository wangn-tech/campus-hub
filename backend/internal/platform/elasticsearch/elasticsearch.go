package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	es "github.com/elastic/go-elasticsearch/v9"
	"github.com/wangn-tech/campus-hub/internal/config"
)

type Client struct{ *es.Client }

// SearchQuery is the public-activity query supported by the ES read model.
// The MySQL service applies the same filters when this dependency is down.
type SearchQuery struct {
	Keyword, CategoryID, TagID, Location, Sort string
	StartTime, EndTime                         *int64
	Status                                     *int
	Longitude, Latitude                        *float64
	Distance                                   string
	Page, PageSize                             int
}

type SearchResult struct {
	IDs   []string
	Total int64
}

func Open(cfg config.ElasticsearchConfig) (*Client, error) {
	transport := &http.Transport{ResponseHeaderTimeout: cfg.RequestTimeout}
	client, err := es.NewClient(es.Config{
		Addresses:         cfg.Addresses,
		Username:          cfg.Username,
		Password:          cfg.Password,
		Transport:         transport,
		DisableRetry:      true,
		EnableMetrics:     false,
		EnableDebugLogger: false,
	})
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

func (c *Client) Check(ctx context.Context) error {
	response, err := c.Info(c.Info.WithContext(ctx))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("elasticsearch returned status %d", response.StatusCode)
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error { return c.Client.Close(ctx) }

// EnsureActivityIndex creates the initial physical index and assigns the
// configured read alias. It is idempotent and intentionally uses standard
// analyzers so local Compose needs no optional IK plugin.
func (c *Client) EnsureActivityIndex(ctx context.Context, index, alias string) error {
	if err := c.CreateActivityIndex(ctx, index); err != nil {
		return err
	}
	response, err := c.request(ctx, http.MethodGet, "/_alias/"+url.PathEscape(alias), nil)
	if err != nil {
		return err
	}
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode != http.StatusNotFound {
		return checkResponse(response)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		return nil
	}
	return c.addAlias(ctx, alias, index)
}

func (c *Client) CreateActivityIndex(ctx context.Context, index string) error {
	body := activityMapping
	response, err := c.request(ctx, http.MethodPut, "/"+url.PathEscape(index), body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusBadRequest {
		if err := checkResponse(response); err != nil {
			return err
		}
	} else {
		response.Body.Close()
	}
	return nil
}

// SwitchAlias atomically detaches all prior targets and attaches the new
// physical index. A failed rebuild never invokes this method, leaving search
// on the previous read model.
func (c *Client) SwitchAlias(ctx context.Context, alias, index string) error {
	body, err := json.Marshal(map[string]any{"actions": []any{
		// `ignore_unavailable` is not accepted by Elasticsearch's remove-alias
		// action. `must_exist: false` keeps the first alias switch idempotent.
		map[string]any{"remove": map[string]any{"index": "*", "alias": alias, "must_exist": false}},
		map[string]any{"add": map[string]string{"index": index, "alias": alias}},
	}})
	if err != nil {
		return err
	}
	response, err := c.request(ctx, http.MethodPost, "/_aliases", body)
	if err != nil {
		return err
	}
	return checkResponse(response)
}

// Refresh makes all completed writes searchable before a rebuild is checked.
func (c *Client) Refresh(ctx context.Context, index string) error {
	response, err := c.request(ctx, http.MethodPost, "/"+url.PathEscape(index)+"/_refresh", nil)
	if err != nil {
		return err
	}
	return checkResponse(response)
}

// Count returns the number of documents in an index or alias.
func (c *Client) Count(ctx context.Context, index string) (int64, error) {
	response, err := c.request(ctx, http.MethodPost, "/"+url.PathEscape(index)+"/_count", []byte(`{"query":{"match_all":{}}}`))
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("elasticsearch returned status %d", response.StatusCode)
	}
	var decoded struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return 0, err
	}
	return decoded.Count, nil
}

// HasDocument verifies a deterministic rebuild sample without retrieving it.
func (c *Client) HasDocument(ctx context.Context, index, id string) (bool, error) {
	response, err := c.request(ctx, http.MethodHead, "/"+url.PathEscape(index)+"/_doc/"+url.PathEscape(id), nil)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Errorf("elasticsearch returned status %d", response.StatusCode)
	}
	return true, nil
}

func (c *Client) addAlias(ctx context.Context, alias, index string) error {
	body, err := json.Marshal(map[string]any{"actions": []any{map[string]any{"add": map[string]string{"index": index, "alias": alias}}}})
	if err != nil {
		return err
	}
	response, err := c.request(ctx, http.MethodPost, "/_aliases", body)
	if err != nil {
		return err
	}
	return checkResponse(response)
}

var activityMapping = []byte(`{"settings":{"number_of_shards":1,"number_of_replicas":0},"mappings":{"properties":{"id":{"type":"keyword"},"title":{"type":"text"},"title_keyword":{"type":"keyword"},"description":{"type":"text"},"category_id":{"type":"keyword"},"category_name":{"type":"keyword"},"tag_ids":{"type":"keyword"},"tag_names":{"type":"text"},"organizer_id":{"type":"keyword"},"organizer_name":{"type":"text"},"organizer_avatar":{"type":"keyword","index":false},"location":{"type":"text","fields":{"keyword":{"type":"keyword"}}},"address_detail":{"type":"text"},"geo_point":{"type":"geo_point"},"status":{"type":"byte"},"register_start_at":{"type":"date","format":"epoch_millis"},"register_end_at":{"type":"date","format":"epoch_millis"},"activity_start_at":{"type":"date","format":"epoch_millis"},"activity_end_at":{"type":"date","format":"epoch_millis"},"max_participants":{"type":"integer"},"approved_participant_count":{"type":"integer"},"pending_participant_count":{"type":"integer"},"view_count":{"type":"long"},"version":{"type":"long"},"deleted":{"type":"boolean"},"created_at":{"type":"date","format":"epoch_millis"},"updated_at":{"type":"date","format":"epoch_millis"}}}}`)

// Index writes a document using its activity version as an external version so
// an older Kafka record cannot overwrite a newer state. external_gte also
// makes the post-alias-switch compensation scan idempotent when it replays an
// unchanged document at the same version.
func (c *Client) Index(ctx context.Context, alias, id string, version uint32, document any) error {
	body, err := json.Marshal(document)
	if err != nil {
		return err
	}
	if version == 0 {
		version = 1
	}
	path := "/" + url.PathEscape(alias) + "/_doc/" + url.PathEscape(id) + "?version_type=external_gte&version=" + strconv.FormatUint(uint64(version), 10)
	response, err := c.request(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	return checkResponse(response)
}

func (c *Client) Search(ctx context.Context, alias string, query SearchQuery) (SearchResult, error) {
	statuses := []int{2, 3, 4}
	if query.Status != nil {
		statuses = []int{*query.Status}
	}
	filters := []any{
		map[string]any{"term": map[string]any{"deleted": false}},
		map[string]any{"terms": map[string]any{"status": statuses}},
	}
	if query.CategoryID != "" {
		filters = append(filters, map[string]any{"term": map[string]string{"category_id": query.CategoryID}})
	}
	if query.TagID != "" {
		filters = append(filters, map[string]any{"term": map[string]string{"tag_ids": query.TagID}})
	}
	if query.Location != "" {
		filters = append(filters, map[string]any{"match": map[string]string{"location": query.Location}})
	}
	if query.StartTime != nil || query.EndTime != nil {
		rangeQuery := map[string]any{}
		if query.StartTime != nil {
			rangeQuery["gte"] = *query.StartTime
		}
		if query.EndTime != nil {
			rangeQuery["lte"] = *query.EndTime
		}
		filters = append(filters, map[string]any{"range": map[string]any{"activity_start_at": rangeQuery}})
	}
	if query.Distance != "" && query.Longitude != nil && query.Latitude != nil {
		filters = append(filters, map[string]any{"geo_distance": map[string]any{"distance": query.Distance, "geo_point": map[string]float64{"lon": *query.Longitude, "lat": *query.Latitude}}})
	}
	boolQuery := map[string]any{"filter": filters}
	if strings.TrimSpace(query.Keyword) != "" {
		boolQuery["must"] = []any{map[string]any{"multi_match": map[string]any{"query": query.Keyword, "fields": []string{"title^4", "description", "tag_names^2", "location^2", "address_detail"}}}}
	}
	sort := []any{map[string]any{"activity_start_at": "asc"}}
	if query.Sort == "created_at" {
		sort = []any{map[string]any{"created_at": "desc"}}
	}
	if query.Sort == "hot" {
		sort = []any{map[string]any{"view_count": "desc"}}
	}
	if query.Sort == "distance" && query.Longitude != nil && query.Latitude != nil {
		sort = []any{map[string]any{"_geo_distance": map[string]any{"geo_point": map[string]float64{"lon": *query.Longitude, "lat": *query.Latitude}, "order": "asc", "unit": "m", "mode": "min", "distance_type": "arc"}}}
	}
	body, err := json.Marshal(map[string]any{"from": (query.Page - 1) * query.PageSize, "size": query.PageSize, "track_total_hits": true, "_source": []string{"id"}, "query": map[string]any{"bool": boolQuery}, "sort": sort})
	if err != nil {
		return SearchResult{}, err
	}
	response, err := c.request(ctx, http.MethodPost, "/"+url.PathEscape(alias)+"/_search", body)
	if err != nil {
		return SearchResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		return SearchResult{}, fmt.Errorf("elasticsearch returned status %d", response.StatusCode)
	}
	var decoded struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source struct {
					ID string `json:"id"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return SearchResult{}, err
	}
	result := SearchResult{Total: decoded.Hits.Total.Value, IDs: make([]string, 0, len(decoded.Hits.Hits))}
	for _, hit := range decoded.Hits.Hits {
		if hit.Source.ID != "" {
			result.IDs = append(result.IDs, hit.Source.ID)
		}
	}
	return result, nil
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	return c.Perform(request)
}

func checkResponse(response *http.Response) error {
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("elasticsearch returned status %d", response.StatusCode)
	}
	return nil
}
