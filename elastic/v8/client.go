package v8

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/pteich/elastic-query-export/elastic"
)

type Client struct {
	client *elasticsearch.Client
}

type ScrollService struct {
	client        *elasticsearch.Client
	index         string
	size          int
	query         map[string]interface{}
	includeFields []string
	scrollID      string
	scrollTime    time.Duration
}

type SearchResult struct {
	hits  []SearchHit
	total int64
}

type SearchHit struct {
	source []byte
}

type QueryBuilder struct {
	query map[string]interface{}
}

func NewClient(cfg elasticsearch.Config) (*Client, error) {
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

func (c *Client) Count(ctx context.Context, index string, query elastic.Query) (int64, error) {
	var buf bytes.Buffer
	if query != nil {
		queryMap := query.Build()
		if len(queryMap) > 0 {
			if err := json.NewEncoder(&buf).Encode(queryMap); err != nil {
				return 0, err
			}
		}
	}

	req := esapi.CountRequest{
		Index: []string{index},
		Body:  &buf,
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, errors.New(res.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, err
	}

	return int64(resp["count"].(float64)), nil
}

func (c *Client) Scroll(index string, size int, query elastic.Query) *ScrollService {
	return &ScrollService{
		client:     c.client,
		index:      index,
		size:       size,
		query:      query.Build(),
		scrollTime: 5 * time.Minute,
	}
}

func (c *Client) Stop() {}

func (s *ScrollService) Do(ctx context.Context) (*SearchResult, error) {
	var buf bytes.Buffer
	var res *esapi.Response
	var err error

	queryBody := make(map[string]interface{})
	if s.query != nil && len(s.query) > 0 {
		queryBody["query"] = s.query
	}
	if len(s.includeFields) > 0 {
		queryBody["_source"] = s.includeFields
	}

	if len(queryBody) > 0 {
		if err := json.NewEncoder(&buf).Encode(queryBody); err != nil {
			return nil, err
		}
	}

	if s.scrollID == "" {
		req := esapi.SearchRequest{
			Index:  []string{s.index},
			Size:   &s.size,
			Scroll: s.scrollTime,
			Body:   &buf,
		}

		res, err = req.Do(ctx, s.client)
	} else {
		req := esapi.ScrollRequest{
			ScrollID: s.scrollID,
			Scroll:   s.scrollTime,
		}
		res, err = req.Do(ctx, s.client)
	}

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, errors.New(res.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	hits, ok := resp["hits"].(map[string]interface{})
	if !ok {
		return &SearchResult{}, nil
	}

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		return &SearchResult{}, nil
	}

	result := &SearchResult{
		hits: make([]SearchHit, 0, len(hitsList)),
	}

	for _, hit := range hitsList {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"]
		if ok {
			sourceBytes, _ := json.Marshal(source)
			result.hits = append(result.hits, SearchHit{source: sourceBytes})
		}

		if s.scrollID == "" {
			if scrollID, ok := resp["_scroll_id"].(string); ok {
				s.scrollID = scrollID
			}
		}
	}

	if total, ok := hits["total"].(map[string]interface{}); ok {
		if value, ok := total["value"].(float64); ok {
			result.total = int64(value)
		}
	}

	return result, nil
}

func (s *ScrollService) Clear(ctx context.Context) error {
	if s.scrollID == "" {
		return nil
	}

	req := esapi.ClearScrollRequest{
		ScrollID: []string{s.scrollID},
	}

	res, err := req.Do(ctx, s.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	s.scrollID = ""
	return nil
}

func (s *ScrollService) FetchSourceContext(includeFields []string) *ScrollService {
	s.includeFields = includeFields
	return s
}

func (r *SearchResult) Hits() []SearchHit {
	return r.hits
}

func (r *SearchResult) Total() int64 {
	return r.total
}

func (h *SearchHit) GetSource() []byte {
	return h.source
}

type BoolQuery struct {
	builder *QueryBuilder
}

type RangeQuery struct {
	builder *QueryBuilder
	field   string
}

type QueryStringQuery struct {
	builder *QueryBuilder
	query   string
}

type MatchAllQuery struct {
	builder *QueryBuilder
}

type RawStringQuery struct {
	builder *QueryBuilder
	raw     string
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		query: make(map[string]interface{}),
	}
}

func NewBoolQuery() *BoolQuery {
	return &BoolQuery{
		builder: NewQueryBuilder(),
	}
}

func (q *BoolQuery) Must(query interface{}) *BoolQuery {
	if q.builder.query["bool"] == nil {
		q.builder.query["bool"] = make(map[string]interface{})
	}
	boolQuery := q.builder.query["bool"].(map[string]interface{})
	if boolQuery["must"] == nil {
		boolQuery["must"] = []interface{}{}
	}
	boolQuery["must"] = append(boolQuery["must"].([]interface{}), query)
	return q
}

func (q *BoolQuery) Filter(query interface{}) *BoolQuery {
	if q.builder.query["bool"] == nil {
		q.builder.query["bool"] = make(map[string]interface{})
	}
	boolQuery := q.builder.query["bool"].(map[string]interface{})
	if boolQuery["filter"] == nil {
		boolQuery["filter"] = []interface{}{}
	}
	boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), query)
	return q
}

func (q *BoolQuery) Build() map[string]interface{} {
	return q.builder.Build()
}

func NewRangeQuery(field string) *RangeQuery {
	return &RangeQuery{
		builder: NewQueryBuilder(),
		field:   field,
	}
}

func (q *RangeQuery) Gte(value string) *RangeQuery {
	if q.builder.query["range"] == nil {
		q.builder.query["range"] = make(map[string]interface{})
	}
	rangeQuery := q.builder.query["range"].(map[string]interface{})
	if rangeQuery[q.field] == nil {
		rangeQuery[q.field] = make(map[string]interface{})
	}
	fieldQuery := rangeQuery[q.field].(map[string]interface{})
	fieldQuery["gte"] = value
	return q
}

func (q *RangeQuery) Lte(value string) *RangeQuery {
	if q.builder.query["range"] == nil {
		q.builder.query["range"] = make(map[string]interface{})
	}
	rangeQuery := q.builder.query["range"].(map[string]interface{})
	if rangeQuery[q.field] == nil {
		rangeQuery[q.field] = make(map[string]interface{})
	}
	fieldQuery := rangeQuery[q.field].(map[string]interface{})
	fieldQuery["lte"] = value
	return q
}

func (q *RangeQuery) Build() map[string]interface{} {
	return q.builder.Build()
}

func NewQueryStringQuery(query string) *QueryStringQuery {
	return &QueryStringQuery{
		builder: NewQueryBuilder(),
		query:   query,
	}
}

func (q *QueryStringQuery) Build() map[string]interface{} {
	q.builder.query["query_string"] = map[string]interface{}{
		"query": q.query,
	}
	return q.builder.Build()
}

func NewMatchAllQuery() *MatchAllQuery {
	return &MatchAllQuery{
		builder: NewQueryBuilder(),
	}
}

func (q *MatchAllQuery) Build() map[string]interface{} {
	q.builder.query["match_all"] = map[string]interface{}{}
	return q.builder.Build()
}

func NewRawStringQuery(rawQuery string) *RawStringQuery {
	builder := NewQueryBuilder()
	if err := json.Unmarshal([]byte(rawQuery), &builder.query); err != nil {
		return nil
	}
	return &RawStringQuery{
		builder: builder,
		raw:     rawQuery,
	}
}

func (q *RawStringQuery) Build() map[string]interface{} {
	return q.builder.Build()
}

func GetDefaultLogger() *log.Logger {
	return log.New(os.Stderr, "ELASTIC ", log.LstdFlags)
}

func CreateLogger(prefix string) *log.Logger {
	return log.New(os.Stderr, prefix, log.LstdFlags)
}

func CreateHTTPClient(tlsConfig interface{}, httpClient *http.Client) *http.Client {
	return httpClient
}

func NewConfig(url string, username string, password string, verifySSL bool, httpClient *http.Client) elasticsearch.Config {
	cfg := elasticsearch.Config{
		Addresses: []string{url},
		Username:  username,
		Password:  password,
		Transport: httpClient.Transport,
	}
	return cfg
}

func (q *QueryBuilder) Build() map[string]interface{} {
	return q.query
}

func TrimSpace(s string) string {
	return strings.TrimSpace(s)
}
