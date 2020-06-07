package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/jawher/mow.cli"
	"github.com/olivere/elastic"
	"golang.org/x/sync/errgroup"
)

var Version string

func main() {
	app := cli.App("elastic-query-export", "CLI tool to export data from ElasticSearch into a CSV file. https://github.com/pteich/elastic-query-export")
	app.Version("v version", Version)

	var (
		configElasticURL = app.StringOpt("c connect", "http://localhost:9200", "ElasticSearch URL")
		configIndex      = app.StringOpt("i index", "logs-*", "ElasticSearch Index (or Index Prefix)")
		configRawQuery   = app.StringOpt("r rawquery", "", "ElasticSearch Raw Querystring")
		configQuery      = app.StringOpt("q query", "*", "Lucene Query like in Kibana search input")
		configOutfile    = app.StringOpt("o outfile", "output.csv", "Filepath for CSV output")
		configStartdate  = app.StringOpt("e start", "", "Start date for documents to include")
		configEnddate    = app.StringOpt("e end", "", "End date for documents to include")
		configScrollsize = app.IntOpt("size", 1000, "Number of documents that will be returned per shard")
		configTimefield  = app.StringOpt("timefield", "Timestamp", "Field name to use for start and end date query")
		configFieldlist  = app.StringOpt("fields", "", "Fields to include in export as comma separated list")
		configFields     = app.StringsOpt("f field", nil, "Field to include in export, can be added multiple for every field")
	)

	app.Action = func() {

		client, err := elastic.NewClient(
			elastic.SetURL(*configElasticURL),
			elastic.SetSniff(false),
			elastic.SetHealthcheckInterval(60*time.Second),
			elastic.SetErrorLog(log.New(os.Stderr, "ELASTIC ", log.LstdFlags)),
		)
		if err != nil {
			log.Printf("Error connecting to ElasticSearch %s - %v", *configElasticURL, err)
			os.Exit(1)
		}
		defer client.Stop()

		if *configFieldlist != "" {
			*configFields = strings.Split(*configFieldlist, ",")
		}

		outfile, err := os.Create(*configOutfile)
		if err != nil {
			log.Printf("Error creating output file - %v", err)
		}
		defer outfile.Close()

		g, ctx := errgroup.WithContext(context.Background())

		var rangeQuery *elastic.RangeQuery

		esQuery := elastic.NewBoolQuery()

		if *configStartdate != "" && *configEnddate != "" {
			rangeQuery = elastic.NewRangeQuery(*configTimefield).Gte(*configStartdate).Lte(*configEnddate)
		} else if *configStartdate != "" {
			rangeQuery = elastic.NewRangeQuery(*configTimefield).Gte(*configStartdate)
		} else if *configEnddate != "" {
			rangeQuery = elastic.NewRangeQuery(*configTimefield).Lte(*configEnddate)
		} else {
			rangeQuery = nil
		}

		if rangeQuery != nil {
			esQuery = esQuery.Filter(rangeQuery)
		}

		if *configRawQuery != "" {
			esQuery = esQuery.Must(elastic.NewRawStringQuery(*configRawQuery))
		} else if *configQuery != "" {
			esQuery = esQuery.Must(elastic.NewQueryStringQuery(*configQuery))
		} else {
			esQuery = esQuery.Must(elastic.NewMatchAllQuery())
		}

		source, _ := esQuery.Source()
		data, _ := json.Marshal(source)
		fmt.Println(string(data))

		// Count total and setup progress
		total, err := client.Count(*configIndex).Query(esQuery).Do(ctx)
		if err != nil {
			log.Printf("Error counting ElasticSearch documents - %v", err)
		}
		bar := pb.StartNew(int(total))

		// one goroutine to receive hits from Elastic and send them to hits channel
		hits := make(chan json.RawMessage)
		g.Go(func() error {
			defer close(hits)

			scroll := client.Scroll(*configIndex).Size(*configScrollsize).Query(esQuery)

			// include selected fields otherwise export all
			if *configFields != nil {
				fetchSource := elastic.NewFetchSourceContext(true)
				for _, field := range *configFields {
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
					hits <- *hit.Source
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
			for _, field := range *configFields {
				csvheader = append(csvheader, field)
			}
			if err := w.Write(csvheader); err != nil {
				log.Printf("Error writing CSV header - %v", err)
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

					for _, field := range *configFields {

						switch reflect.TypeOf(document[field]).String() {
						case "int64":
							outdata = fmt.Sprintf("%d", document[field])
						case "float64":
							outdata = fmt.Sprintf("%f", document[field])
						default:
							outdata = fmt.Sprintf("%v", document[field])
						}

						csvdata = append(csvdata, outdata)
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

	app.Run(os.Args)
}
