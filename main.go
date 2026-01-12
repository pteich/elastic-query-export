package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pteich/configstruct"

	"github.com/pteich/elastic-query-export/export"
	"github.com/pteich/elastic-query-export/flags"
	"github.com/pteich/elastic-query-export/tui"
)

var Version string

const (
	configEnvVar          = "ELASTIC_QUERY_EXPORT_CONFIG"
	defaultConfigFileName = ".elastic-query-export.yaml"
)

func main() {
	conf := flags.Flags{
		ElasticURL:       "https://localhost:9200",
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
		if configPath, ok := resolveConfigPath(); ok {
			if err := configstruct.Parse(&conf, configstruct.WithYamlConfig(configPath)); err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				os.Exit(1)
			}
			conf.ConfigPath = configPath
		}

		p := tea.NewProgram(tui.InitialModel(&conf))
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if conf.ConfigPath != "" {
			if err := configstruct.Save(conf.ConfigPath, &conf); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				os.Exit(1)
			}
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

func resolveConfigPath() (string, bool) {
	if configPath, ok := os.LookupEnv(configEnvVar); ok && configPath != "" {
		return configPath, true
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	defaultPath := filepath.Join(homeDir, defaultConfigFileName)
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, true
	}

	return "", false
}
