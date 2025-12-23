package elastic

import "context"

type Client interface {
	Count(ctx context.Context, index string, query Query) (int64, error)
	Scroll(index string, size int, query Query) ScrollService
	Stop()
}

type Query interface {
	Build() map[string]interface{}
}

type ScrollService interface {
	Do(ctx context.Context) (SearchResult, error)
	Clear(ctx context.Context) error
	FetchSourceContext(includeFields []string) ScrollService
}

type SearchResult interface {
	Hits() []SearchHit
	Total() int64
}

type SearchHit interface {
	GetSource() []byte
}
