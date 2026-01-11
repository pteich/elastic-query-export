package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pteich/configstruct"

	"github.com/pteich/elastic-query-export/export"
	"github.com/pteich/elastic-query-export/flags"
	"github.com/pteich/elastic-query-export/tui"
)

var Version string

func main() {
	conf := flags.Flags{
		ElasticURL:       "http://localhost:9200",
		ElasticVerifySSL: false,
		ElasticVersion:   7,
		Index:            "logs-*",
		Query:            "*",
		OutFormat:        flags.FormatCSV,
		Outfile:          "output.csv",
		ScrollSize:       1000,
		Timefield:        "@timestamp",
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if len(os.Args) < 2 {
		p := tea.NewProgram(tui.InitialModel(&conf))
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cmd := configstruct.NewCommand(
		"",
		"CLI tool to export data from ElasticSearch into a CSV or JSON file. https://github.com/pteich/elastic-query-export",
		&conf,
		func(c *configstruct.Command, cfg interface{}) error {
			export.Run(ctx, cfg.(*flags.Flags))
			return nil
		},
	)

	err := cmd.ParseAndRun(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
