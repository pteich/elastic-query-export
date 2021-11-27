package formats

import (
	"context"
	"fmt"
	"os"

	"github.com/olivere/elastic/v7"
	"gopkg.in/cheggaaa/pb.v2"
)

type JSON struct {
	Outfile    *os.File
	ProgessBar *pb.ProgressBar
}

func (j JSON) Run(ctx context.Context, hits <-chan *elastic.SearchHit) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case hit := <-hits:
			fmt.Fprintln(j.Outfile, string(hit.Source))
			j.ProgessBar.Increment()
		}
	}
}
