package formats

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/cheggaaa/pb.v2"

	"github.com/pteich/elastic-query-export/flags"
)

type JSON struct {
	Conf       *flags.Flags
	Outfile    *os.File
	ProgessBar *pb.ProgressBar
}

func (j JSON) Run(ctx context.Context, hits <-chan json.RawMessage) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case hit := <-hits:
			fmt.Fprintln(j.Outfile, hit)
		}
	}
}
