package main

import (
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/olivere/elastic/v7"
	"golang.org/x/sync/errgroup"
	"gopkg.in/cheggaaa/pb.v2"
)

func export(ctx context.Context, conf *Flags) {
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
		conf.fields = strings.Split(conf.Fieldlist, ",")
	}

	outfile, err := os.Create(conf.Outfile)
	if err != nil {
		log.Fatalf("Error creating output file - %s", err)
	}
	defer outfile.Close()

	g, ctx := errgroup.WithContext(ctx)

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

	/*
		source, _ := esQuery.Source()
		data, _ := json.Marshal(source)
		fmt.Println(string(data))
	*/

	// Count total and setup progress
	total, err := client.Count(conf.Index).Query(esQuery).Do(ctx)
	if err != nil {
		log.Printf("Error counting ElasticSearch documents - %v", err)
	}
	bar := pb.StartNew(int(total))

	// one goroutine to receive hits from Elastic and send them to hits channel
	hits := make(chan json.RawMessage)
	g.Go(func() error {
		defer close(hits)

		scroll := client.Scroll(conf.Index).Size(conf.ScrollSize).Query(esQuery)

		// include selected fields otherwise export all
		if conf.fields != nil {
			fetchSource := elastic.NewFetchSourceContext(true)
			for _, field := range conf.fields {
				fetchSource.Include(field)
			}
			scroll = scroll.FetchSourceContext(fetchSource)
		}

		for {
			results, err := scroll.Do(ctx)
			if err == io.EOF {
				return nil // all results retrieved
			}
			if err != nil {
				return err // something went wrong
			}

			// Send the hits to the hits channel
			for _, hit := range results.Hits.Hits {
				hits <- hit.Source
			}

			// Check if we need to terminate early
			select {
			default:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	// goroutine outside of the errgroup to receive csv outputs from csvout channel and write to file
	csvout := make(chan []string, 8)
	go func() {
		w := csv.NewWriter(outfile)

		var csvheader []string
		if conf.fields != nil {
			for _, field := range conf.fields {
				csvheader = append(csvheader, field)
			}
			if err := w.Write(csvheader); err != nil {
				log.Printf("Error writing CSV header - %v", err)
			}
		}

		for csvdata := range csvout {
			if err := w.Write(csvdata); err != nil {
				log.Printf("Error writing CSV data - %v", err)
			}

			w.Flush()
			bar.Increment()
		}

	}()

	// some more goroutines in the errgroup context to do the transformation, room to add more work here in future
	for i := 0; i < 8; i++ {
		g.Go(func() error {
			var document map[string]interface{}

			for hit := range hits {
				var csvdata []string
				var outdata string

				if err := json.Unmarshal(hit, &document); err != nil {
					log.Printf("Error unmarshal JSON from ElasticSearch - %v", err)
				}

				if conf.fields != nil {
					for _, field := range conf.fields {
						if val, ok := document[field]; ok {
							if val == nil {
								csvdata = append(csvdata, "")
								continue
							}

							// this type switch is probably not really needed anymore
							switch val.(type) {
							case int64:
								outdata = fmt.Sprintf("%d", val)
							case float64:
								f := val.(float64)
								d := int(f)
								if f == float64(d) {
									outdata = fmt.Sprintf("%d", d)
								} else {
									outdata = fmt.Sprintf("%f", f)
								}

							default:
								outdata = removeLBR(fmt.Sprintf("%v", val))
							}

							csvdata = append(csvdata, outdata)
						} else {
							csvdata = append(csvdata, "")
						}
					}

				} else {
					for _, val := range document {
						outdata = removeLBR(fmt.Sprintf("%v", val))
						csvdata = append(csvdata, outdata)
					}
				}

				// send string array to csv output
				csvout <- csvdata

				select {
				default:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	// Check if any goroutines failed.
	if err := g.Wait(); err != nil {
		log.Printf("Error - %v", err)
	}

	bar.Finish()
}

func removeLBR(text string) string {
	re := regexp.MustCompile(`\x{000D}\x{000A}|[\x{000A}\x{000B}\x{000C}\x{000D}\x{0085}\x{2028}\x{2029}]`)
	return re.ReplaceAllString(text, ``)
}
