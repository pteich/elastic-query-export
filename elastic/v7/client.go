package v7

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/olivere/elastic/v7"
)

type Client struct {
	client *elastic.Client
}

type ScrollService struct {
	scroll *elastic.ScrollService
}

type SearchResult struct {
	results *elastic.SearchResult
}

type SearchHit struct {
	hit *elastic.SearchHit
}

func NewClient(esOpts []elastic.ClientOptionFunc) (*Client, error) {
	client, err := elastic.NewClient(esOpts...)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

func (c *Client) Count(ctx context.Context, index string, query elastic.Query) (int64, error) {
	count, err := c.client.Count(index).Query(query).Do(ctx)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (c *Client) Scroll(index string, size int, query elastic.Query) *ScrollService {
	return &ScrollService{
		scroll: c.client.Scroll(index).Size(size).Query(query),
	}
}

func (c *Client) Stop() {
	c.client.Stop()
}

func (s *ScrollService) Do(ctx context.Context) (*SearchResult, error) {
	results, err := s.scroll.Do(ctx)
	if err != nil {
		return nil, err
	}
	return &SearchResult{results: results}, nil
}

func (s *ScrollService) Clear(ctx context.Context) error {
	return s.scroll.Clear(ctx)
}

func (s *ScrollService) FetchSourceContext(includeFields []string) *ScrollService {
	fsc := elastic.NewFetchSourceContext(true)
	for _, field := range includeFields {
		fsc.Include(field)
	}
	return &ScrollService{
		scroll: s.scroll.FetchSourceContext(fsc),
	}
}

func (r *SearchResult) Hits() []*SearchHit {
	hits := make([]*SearchHit, len(r.results.Hits.Hits))
	for i, hit := range r.results.Hits.Hits {
		hits[i] = &SearchHit{hit: hit}
	}
	return hits
}

func (h *SearchHit) GetSource() []byte {
	return h.hit.Source
}

func (h *SearchHit) Unwrap() *elastic.SearchHit {
	return h.hit
}

func NewBoolQuery() *elastic.BoolQuery {
	return elastic.NewBoolQuery()
}

func NewRangeQuery(field string) *elastic.RangeQuery {
	return elastic.NewRangeQuery(field)
}

func NewQueryStringQuery(query string) *elastic.QueryStringQuery {
	return elastic.NewQueryStringQuery(query)
}

func NewMatchAllQuery() *elastic.MatchAllQuery {
	return elastic.NewMatchAllQuery()
}

func NewRawStringQuery(rawQuery string) elastic.RawStringQuery {
	return elastic.NewRawStringQuery(rawQuery)
}

func SetHttpClient(httpClient *http.Client) elastic.ClientOptionFunc {
	return elastic.SetHttpClient(httpClient)
}

func SetURL(urls ...string) elastic.ClientOptionFunc {
	return elastic.SetURL(urls...)
}

func SetSniff(enabled bool) elastic.ClientOptionFunc {
	return elastic.SetSniff(enabled)
}

func SetHealthcheckInterval(interval time.Duration) elastic.ClientOptionFunc {
	return elastic.SetHealthcheckInterval(interval)
}

func SetErrorLog(logger *log.Logger) elastic.ClientOptionFunc {
	return elastic.SetErrorLog(logger)
}

func SetTraceLog(logger *log.Logger) elastic.ClientOptionFunc {
	return elastic.SetTraceLog(logger)
}

func SetBasicAuth(username, password string) elastic.ClientOptionFunc {
	return elastic.SetBasicAuth(username, password)
}

func GetDefaultLogger() *log.Logger {
	return log.New(os.Stderr, "ELASTIC ", log.LstdFlags)
}
