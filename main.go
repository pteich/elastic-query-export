package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pteich/configstruct"
)

var Version string

func main() {
	flags := Flags{
		ElasticURL:       "http://localhost:9200",
		ElasticVerifySSL: true,
		Index:            "logs-*",
		Query:            "*",
		Outfile:          "output.csv",
		ScrollSize:       1000,
		Timefield:        "@timestamp",
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cmd := configstruct.NewCommand(
		"",
		"CLI tool to export data from ElasticSearch into a CSV file. https://github.com/pteich/elastic-query-export",
		&flags,
		func(c *configstruct.Command, cfg interface{}) error {
			export(ctx, cfg.(*Flags))
			return nil
		},
	)

	err := cmd.ParseAndRun(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
