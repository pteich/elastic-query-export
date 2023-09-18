package formats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/olivere/elastic/v7"
	"gopkg.in/cheggaaa/pb.v2"
)

type Raw struct {
	Outfile    *os.File
	ProgessBar *pb.ProgressBar
}

func (r Raw) Run(ctx context.Context, hits <-chan *elastic.SearchHit) error {
	for hit := range hits {
		data, err := json.Marshal(hit)
		if err != nil {
			log.Println(err)
			continue
		}
		fmt.Fprintln(r.Outfile, string(data))
		r.ProgessBar.Increment()
	}
	// When the hits channel is closed, exit the loop gracefully.
	return nil
}
