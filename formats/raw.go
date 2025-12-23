package formats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gopkg.in/cheggaaa/pb.v2"

	"github.com/pteich/elastic-query-export/elastic"
)

type Raw struct {
	Outfile    *os.File
	ProgessBar *pb.ProgressBar
}

func (r Raw) Run(ctx context.Context, hits <-chan elastic.SearchHit) error {
	for hit := range hits {
		data, err := json.Marshal(hit)
		if err != nil {
			log.Println(err)
			continue
		}

		fmt.Fprintln(r.Outfile, string(data))
		r.ProgessBar.Increment()
	}

	return nil
}
