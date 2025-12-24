package export

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	elasticv7import "github.com/olivere/elastic/v7"
	elasticsearch "github.com/pteich/elastic-query-export/elastic"
	elasticv7 "github.com/pteich/elastic-query-export/elastic/v7"
	elasticv8 "github.com/pteich/elastic-query-export/elastic/v8"
	elasticv9 "github.com/pteich/elastic-query-export/elastic/v9"
	"github.com/pteich/elastic-query-export/flags"
	"github.com/pteich/elastic-query-export/formats"
	"gopkg.in/cheggaaa/pb.v2"
)

const workers = 8

type Formatter interface {
	Run(context.Context, <-chan elasticsearch.SearchHit) error
}

type elasticClient struct {
	version int
	client  any
}

func (e *elasticClient) Count(ctx context.Context, index string, query any) (int64, error) {
	switch e.version {
	case 7:
		client := e.client.(*elasticv7.Client)
		q, ok := query.(elasticv7import.Query)
		if !ok {
			return 0, errors.New("invalid query type for v7")
		}
		return client.Count(ctx, index, q)
	case 8:
		client := e.client.(*elasticv8.Client)
		q, ok := query.(elasticsearch.Query)
		if !ok {
			return 0, errors.New("invalid query type for v8")
		}
		return client.Count(ctx, index, q)
	case 9:
		client := e.client.(*elasticv9.Client)
		q, ok := query.(elasticsearch.Query)
		if !ok {
			return 0, errors.New("invalid query type for v9")
		}
		return client.Count(ctx, index, q)
	default:
		return 0, errors.New("unsupported version")
	}
}

func (e *elasticClient) Scroll(index string, size int, query any) any {
	switch e.version {
	case 7:
		client := e.client.(*elasticv7.Client)
		q, ok := query.(elasticv7import.Query)
		if !ok {
			return nil
		}
		return client.Scroll(index, size, q)
	case 8:
		client := e.client.(*elasticv8.Client)
		q, ok := query.(elasticsearch.Query)
		if !ok {
			return nil
		}
		return client.Scroll(index, size, q)
	case 9:
		client := e.client.(*elasticv9.Client)
		q, ok := query.(elasticsearch.Query)
		if !ok {
			return nil
		}
		return client.Scroll(index, size, q)
	default:
		return nil
	}
}

func (e *elasticClient) Stop() {
	switch e.version {
	case 7:
		client := e.client.(*elasticv7.Client)
		client.Stop()
	case 8:
		client := e.client.(*elasticv8.Client)
		client.Stop()
	case 9:
		client := e.client.(*elasticv9.Client)
		client.Stop()
	}
}

func scrollServiceDo(ctx context.Context, version int, scrollService any) (any, int64, error) {
	switch version {
	case 7:
		scroll := scrollService.(*elasticv7.ScrollService)
		result, err := scroll.Do(ctx)
		if err != nil {
			return nil, 0, err
		}
		return result.Hits(), int64(len(result.Hits())), nil
	case 8:
		scroll := scrollService.(*elasticv8.ScrollService)
		result, err := scroll.Do(ctx)
		if err != nil {
			return nil, 0, err
		}
		return result.Hits(), result.Total(), nil
	case 9:
		scroll := scrollService.(*elasticv9.ScrollService)
		result, err := scroll.Do(ctx)
		if err != nil {
			return nil, 0, err
		}
		return result.Hits(), result.Total(), nil
	default:
		return nil, 0, errors.New("unsupported version")
	}
}

func scrollServiceClear(ctx context.Context, version int, scrollService any) error {
	switch version {
	case 7:
		scroll := scrollService.(*elasticv7.ScrollService)
		return scroll.Clear(ctx)
	case 8:
		scroll := scrollService.(*elasticv8.ScrollService)
		return scroll.Clear(ctx)
	case 9:
		scroll := scrollService.(*elasticv9.ScrollService)
		return scroll.Clear(ctx)
	default:
		return errors.New("unsupported version")
	}
}

func scrollServiceFetchSourceContext(version int, scrollService any, includeFields []string) any {
	switch version {
	case 7:
		scroll := scrollService.(*elasticv7.ScrollService)
		return scroll.FetchSourceContext(includeFields)
	case 8:
		scroll := scrollService.(*elasticv8.ScrollService)
		return scroll.FetchSourceContext(includeFields)
	case 9:
		scroll := scrollService.(*elasticv9.ScrollService)
		return scroll.FetchSourceContext(includeFields)
	default:
		return nil
	}
}

func Run(ctx context.Context, conf *flags.Flags) {
	client, query, err := createClientAndQuery(conf)
	if err != nil {
		log.Fatalf("Error connecting to ElasticSearch: %s", err)
	}
	defer client.Stop()

	if conf.Fieldlist != "" {
		conf.Fields = strings.Split(conf.Fieldlist, ",")
	}

	var outfile *os.File

	if conf.Outfile == "-" {
		outfile = os.Stdout
	} else {
		outfile, err = os.Create(conf.Outfile)
		if err != nil {
			log.Fatalf("Error creating output file - %s", err)
		}
		defer outfile.Close()
	}

	var rangeQuery any

	esQuery := buildBoolQuery(client.version)

	if conf.StartDate != "" && conf.EndDate != "" {
		rangeQuery = buildRangeQuery(client.version, conf.Timefield, conf.StartDate, conf.EndDate, true, true)
	} else if conf.StartDate != "" {
		rangeQuery = buildRangeQuery(client.version, conf.Timefield, conf.StartDate, "", true, false)
	} else if conf.EndDate != "" {
		rangeQuery = buildRangeQuery(client.version, conf.Timefield, "", conf.EndDate, false, true)
	} else {
		rangeQuery = nil
	}

	if rangeQuery != nil {
		esQuery = applyFilterToBoolQuery(client.version, esQuery, rangeQuery)
	}

	if conf.RAWQuery != "" {
		esQuery = applyMustToBoolQuery(client.version, esQuery, buildRawQuery(client.version, conf.RAWQuery))
	} else if conf.Query != "" {
		esQuery = applyMustToBoolQuery(client.version, esQuery, buildQueryStringQuery(client.version, conf.Query))
	} else {
		esQuery = applyMustToBoolQuery(client.version, esQuery, buildMatchAllQuery(client.version))
	}

	query = buildFinalQuery(client.version, esQuery)

	total, err := client.Count(ctx, conf.Index, query)
	if err != nil {
		log.Fatalf("Error counting ElasticSearch documents: %s", err)
	}
	bar := pb.StartNew(int(total))

	hits := make(chan elasticsearch.SearchHit)

	go func() {
		defer close(hits)

		scroll := client.Scroll(conf.Index, conf.ScrollSize, query)
		defer scrollServiceClear(ctx, client.version, scroll)

		if conf.Fields != nil {
			scroll = scrollServiceFetchSourceContext(client.version, scroll, conf.Fields)
		}

		for {
			hitsData, scrollTotal, err := scrollServiceDo(ctx, client.version, scroll)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				log.Println(err)
				return
			}

			switch client.version {
			case 7:
				v7Hits := hitsData.([]*elasticv7.SearchHit)
				for _, hit := range v7Hits {
					select {
					case hits <- &v7SearchHit{hit: hit}:
					case <-ctx.Done():
						return
					}
				}
			case 8:
				v8Hits := hitsData.([]elasticv8.SearchHit)
				for _, hit := range v8Hits {
					select {
					case hits <- &v8SearchHit{hit: &hit}:
					case <-ctx.Done():
						return
					}
				}
			case 9:
				v9Hits := hitsData.([]elasticv9.SearchHit)
				for _, hit := range v9Hits {
					select {
					case hits <- &v9SearchHit{hit: &hit}:
					case <-ctx.Done():
						return
					}
				}
			}

			if scrollTotal == 0 || scrollTotal < int64(conf.ScrollSize) {
				return
			}
		}
	}()

	var output Formatter
	switch conf.OutFormat {
	case flags.FormatJSON:
		output = formats.JSON{
			Outfile:    outfile,
			ProgessBar: bar,
		}
	case flags.FormatRAW:
		output = formats.Raw{
			Outfile:    outfile,
			ProgessBar: bar,
		}
	default:
		output = formats.CSV{
			Conf:       conf,
			Outfile:    outfile,
			Workers:    workers,
			ProgessBar: bar,
		}
	}

	err = output.Run(ctx, hits)
	if err != nil {
		log.Printf("Failed to write output: %s", err)
	}

	bar.Finish()
}

func createClientAndQuery(conf *flags.Flags) (*elasticClient, any, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: !conf.ElasticVerifySSL,
	}

	if conf.ElasticClientCrt != "" && conf.ElasticClientKey != "" {
		cert, err := tls.LoadX509KeyPair(conf.ElasticClientCrt, conf.ElasticClientKey)
		if err != nil {
			return nil, nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	tr := &http.Transport{
		TLSClientConfig: tlsCfg,
	}
	httpClient := &http.Client{Transport: tr}

	logger := elasticv7.GetDefaultLogger()

	switch conf.ElasticVersion {
	case 7:
		esOpts := []elasticv7import.ClientOptionFunc{
			elasticv7.SetHttpClient(httpClient),
			elasticv7.SetURL(conf.ElasticURL),
			elasticv7.SetSniff(false),
			elasticv7.SetHealthcheckInterval(60 * time.Second),
			elasticv7.SetErrorLog(logger),
		}

		if conf.Trace {
			esOpts = append(esOpts, elasticv7.SetTraceLog(logger))
		}

		if conf.ElasticUser != "" && conf.ElasticPass != "" {
			esOpts = append(esOpts, elasticv7.SetBasicAuth(conf.ElasticUser, conf.ElasticPass))
		}

		client, err := elasticv7.NewClient(esOpts)
		if err != nil {
			return nil, nil, err
		}
		return &elasticClient{version: 7, client: client}, elasticv7.NewBoolQuery(), nil

	case 8:
		cfg := elasticv8.NewConfig(conf.ElasticURL, conf.ElasticUser, conf.ElasticPass, conf.ElasticVerifySSL, httpClient)
		client, err := elasticv8.NewClient(cfg)
		if err != nil {
			return nil, nil, err
		}
		return &elasticClient{version: 8, client: client}, elasticv8.NewBoolQuery(), nil

	case 9:
		cfg := elasticv9.NewConfig(conf.ElasticURL, conf.ElasticUser, conf.ElasticPass, conf.ElasticVerifySSL, httpClient)
		client, err := elasticv9.NewClient(cfg)
		if err != nil {
			return nil, nil, err
		}
		return &elasticClient{version: 9, client: client}, elasticv9.NewBoolQuery(), nil

	default:
		return nil, nil, errors.New("unsupported ElasticSearch version")
	}
}

func buildBoolQuery(version int) any {
	switch version {
	case 7:
		return elasticv7.NewBoolQuery()
	case 8:
		return elasticv8.NewBoolQuery()
	case 9:
		return elasticv9.NewBoolQuery()
	default:
		return nil
	}
}

func buildRangeQuery(version int, field, gte, lte string, hasGte, hasLte bool) any {
	switch version {
	case 7:
		rq := elasticv7.NewRangeQuery(field)
		if hasGte {
			rq = rq.Gte(gte)
		}
		if hasLte {
			rq = rq.Lte(lte)
		}
		return rq
	case 8:
		rq := elasticv8.NewRangeQuery(field)
		if hasGte {
			rq = rq.Gte(gte)
		}
		if hasLte {
			rq = rq.Lte(lte)
		}
		return rq
	case 9:
		rq := elasticv9.NewRangeQuery(field)
		if hasGte {
			rq = rq.Gte(gte)
		}
		if hasLte {
			rq = rq.Lte(lte)
		}
		return rq
	default:
		return nil
	}
}

func buildQueryStringQuery(version int, query string) any {
	switch version {
	case 7:
		return elasticv7.NewQueryStringQuery(query)
	case 8:
		return elasticv8.NewQueryStringQuery(query)
	case 9:
		return elasticv9.NewQueryStringQuery(query)
	default:
		return nil
	}
}

func buildMatchAllQuery(version int) any {
	switch version {
	case 7:
		return elasticv7.NewMatchAllQuery()
	case 8:
		return elasticv8.NewMatchAllQuery()
	case 9:
		return elasticv9.NewMatchAllQuery()
	default:
		return nil
	}
}

func buildRawQuery(version int, rawQuery string) any {
	switch version {
	case 7:
		return elasticv7.NewRawStringQuery(rawQuery)
	case 8:
		return elasticv8.NewRawStringQuery(rawQuery)
	case 9:
		return elasticv9.NewRawStringQuery(rawQuery)
	default:
		return nil
	}
}

func applyFilterToBoolQuery(version int, boolQuery any, filter any) any {
	switch version {
	case 7:
		bq := boolQuery.(*elasticv7import.BoolQuery)
		fq, ok := filter.(elasticv7import.Query)
		if !ok {
			return nil
		}
		return bq.Filter(fq)
	case 8:
		bq := boolQuery.(*elasticv8.BoolQuery)
		return bq.Filter(filter)
	case 9:
		bq := boolQuery.(*elasticv9.BoolQuery)
		return bq.Filter(filter)
	default:
		return nil
	}
}

func applyMustToBoolQuery(version int, boolQuery any, must any) any {
	switch version {
	case 7:
		bq := boolQuery.(*elasticv7import.BoolQuery)
		mq, ok := must.(elasticv7import.Query)
		if !ok {
			return nil
		}
		return bq.Must(mq)
	case 8:
		bq := boolQuery.(*elasticv8.BoolQuery)
		return bq.Must(must)
	case 9:
		bq := boolQuery.(*elasticv9.BoolQuery)
		return bq.Must(must)
	default:
		return nil
	}
}

func buildFinalQuery(version int, boolQuery any) any {
	switch version {
	case 7:
		bq := boolQuery.(*elasticv7import.BoolQuery)
		return bq
	case 8:
		return boolQuery.(*elasticv8.BoolQuery)
	case 9:
		return boolQuery.(*elasticv9.BoolQuery)
	default:
		return nil
	}
}

type v7SearchHit struct {
	hit *elasticv7.SearchHit
}

func (h *v7SearchHit) GetSource() []byte {
	return h.hit.GetSource()
}

type v8SearchHit struct {
	hit *elasticv8.SearchHit
}

func (h *v8SearchHit) GetSource() []byte {
	return h.hit.GetSource()
}

type v9SearchHit struct {
	hit *elasticv9.SearchHit
}

func (h *v9SearchHit) GetSource() []byte {
	return h.hit.GetSource()
}
