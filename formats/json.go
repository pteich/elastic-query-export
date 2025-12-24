package formats

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/cheggaaa/pb.v2"

	"github.com/pteich/elastic-query-export/elastic"
)

type JSON struct {
	Outfile    *os.File
	ProgessBar *pb.ProgressBar
}

func (j JSON) Run(ctx context.Context, hits <-chan elastic.SearchHit) error {
	for hit := range hits {
		fmt.Fprintln(j.Outfile, string(hit.GetSource()))
		j.ProgessBar.Increment()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}
