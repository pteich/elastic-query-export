package formats

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"

	"github.com/olivere/elastic/v7"
	"golang.org/x/sync/errgroup"
	"gopkg.in/cheggaaa/pb.v2"

	"github.com/pteich/elastic-query-export/flags"
)

type CSV struct {
	Conf       *flags.Flags
	Outfile    *os.File
	Workers    int
	ProgessBar *pb.ProgressBar
}

func (c CSV) Run(ctx context.Context, hits <-chan *elastic.SearchHit) error {
	g, ctx := errgroup.WithContext(ctx)

	csvout := make(chan []string, c.Workers)
	defer close(csvout)

	go func() {
		w := csv.NewWriter(c.Outfile)

		for csvdata := range csvout {
			if err := w.Write(csvdata); err != nil {
				log.Printf("Error writing CSV data - %v", err)
			}

			w.Flush()
			c.ProgessBar.Increment()
		}

	}()

	sendHeader := sync.Once{}
	fields := c.Conf.Fields
	headerSent := make(chan struct{})

	for i := 0; i < c.Workers; i++ {
		g.Go(func() error {

			for hit := range hits {
				var document map[string]interface{}
				var csvdata []string
				var outdata string

				if err := json.Unmarshal(hit.Source, &document); err != nil {
					log.Printf("Error unmarshal JSON from ElasticSearch - %v", err)
				}

				sendHeader.Do(func() {
					if c.Conf.Fields == nil {
						for key := range document {
							fields = append(fields, key)
						}
					}
					csvout <- fields
					close(headerSent)
				})

				<-headerSent

				for _, field := range fields {
					if val, ok := document[field]; ok {
						if val == nil {
							csvdata = append(csvdata, "")
							continue
						}

						// this type switch is probably not really needed anymore
						switch val := val.(type) {
						case int64:
							outdata = fmt.Sprintf("%d", val)
						case float64:
							d := int(val)
							if val == float64(d) {
								outdata = fmt.Sprintf("%d", d)
							} else {
								outdata = fmt.Sprintf("%f", val)
							}
						default:
							outdata = removeLBR(fmt.Sprintf("%v", val))
						}

						csvdata = append(csvdata, outdata)
					} else {
						csvdata = append(csvdata, "")
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

	return g.Wait()
}

func removeLBR(text string) string {
	re := regexp.MustCompile(`\x{000D}\x{000A}|[\x{000A}\x{000B}\x{000C}\x{000D}\x{0085}\x{2028}\x{2029}]`)
	return re.ReplaceAllString(text, ``)
}
