package export

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/olivere/elastic/v7"
	"gopkg.in/cheggaaa/pb.v2"

	"github.com/pteich/elastic-query-export/flags"
	"github.com/pteich/elastic-query-export/formats"
)

const workers = 8

// Formatter defines how an output formatter has to look like
type Formatter interface {
	Run(context.Context, <-chan *elastic.SearchHit) error
}

// Run starts the export of Elastic data
func Run(ctx context.Context, conf *flags.Flags) {
	exportCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !conf.ElasticVerifySSL},
	}
	httpClient := &http.Client{Transport: tr}

	esOpts := make([]elastic.ClientOptionFunc, 0)
	esOpts = append(esOpts,
		elastic.SetHttpClient(httpClient),
		elastic.SetURL(conf.ElasticURL),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(60*time.Second),
		elastic.SetErrorLog(log.New(os.Stderr, "ELASTIC ", log.LstdFlags)),
	)

	if conf.ElasticUser != "" && conf.ElasticPass != "" {
		esOpts = append(esOpts, elastic.SetBasicAuth(conf.ElasticUser, conf.ElasticPass))
	}

	client, err := elastic.NewClient(esOpts...)
	if err != nil {
		log.Fatalf("Error connecting to ElasticSearch: %s", err)
	}
	defer client.Stop()

	if conf.Fieldlist != "" {
		conf.Fields = strings.Split(conf.Fieldlist, ",")
	}

	outfile, err := os.Create(conf.Outfile)
	if err != nil {
		log.Fatalf("Error creating output file - %s", err)
	}
	defer outfile.Close()

	var rangeQuery *elastic.RangeQuery

	esQuery := elastic.NewBoolQuery()

	if conf.StartDate != "" && conf.EndDate != "" {
		rangeQuery = elastic.NewRangeQuery(conf.Timefield).Gte(conf.StartDate).Lte(conf.EndDate)
	} else if conf.StartDate != "" {
		rangeQuery = elastic.NewRangeQuery(conf.Timefield).Gte(conf.StartDate)
	} else if conf.EndDate != "" {
		rangeQuery = elastic.NewRangeQuery(conf.Timefield).Lte(conf.EndDate)
	} else {
		rangeQuery = nil
	}

	if rangeQuery != nil {
		esQuery = esQuery.Filter(rangeQuery)
	}

	if conf.RAWQuery != "" {
		esQuery = esQuery.Must(elastic.NewRawStringQuery(conf.RAWQuery))
	} else if conf.Query != "" {
		esQuery = esQuery.Must(elastic.NewQueryStringQuery(conf.Query))
	} else {
		esQuery = esQuery.Must(elastic.NewMatchAllQuery())
	}

	// Count total and setup progress
	total, err := client.Count(conf.Index).Query(esQuery).Do(ctx)
	if err != nil {
		log.Printf("Error counting ElasticSearch documents - %v", err)
	}
	bar := pb.StartNew(int(total))

	hits := make(chan *elastic.SearchHit)

	go func() {
		defer close(hits)

		scroll := client.Scroll(conf.Index).Size(conf.ScrollSize).Query(esQuery)

		// include selected fields otherwise export all
		if conf.Fields != nil {
			fetchSource := elastic.NewFetchSourceContext(true)
			for _, field := range conf.Fields {
				fetchSource.Include(field)
			}
			scroll = scroll.FetchSourceContext(fetchSource)
		}

		for {
			results, err := scroll.Do(ctx)
			if err == io.EOF {
				return // all results retrieved
			}
			if err != nil {
				log.Println(err)
				cancel()
				return // something went wrong
			}

			// Send the hits to the hits channel
			for _, hit := range results.Hits.Hits {
				// Check if we need to terminate early
				select {
				case hits <- hit:
				case <-exportCtx.Done():
					return
				}
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

	err = output.Run(exportCtx, hits)
	if err != nil {
		log.Println(err)
	}

	bar.Finish()
}
