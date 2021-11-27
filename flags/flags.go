package flags

const (
	FormatCSV  = "csv"
	FormatJSON = "json"
	FormatRAW  = "raw"
)

type Flags struct {
	ElasticURL       string `cli:"connect" cliAlt:"c" usage:"ElasticSearch URL"`
	ElasticUser      string `cli:"user" usage:"ElasticSearch Username"`
	ElasticPass      string `cli:"pass" usage:"ElasticSearch Password"`
	ElasticVerifySSL bool   `cli:"verifySSL" usage:"Verify SSL certificate"`
	Index            string `cli:"index" cliAlt:"i" usage:"ElasticSearch Index (or Index Prefix)"`
	RAWQuery         string `cli:"rawquery" cliAlt:"r" usage:"ElasticSearch raw query string"`
	Query            string `cli:"query" cliAlt:"q" usage:"Lucene query same that is used in Kibana search input"`
	OutFormat        string `cli:"outformat" cliAlt:"f" usage:"Format of the output data. [json|csv]"`
	Outfile          string `cli:"outfile" cliAlt:"o" usage:"Path to output file"`
	StartDate        string `cli:"start" cliAlt:"s" usage:"Start date for included documents"`
	EndDate          string `cli:"end" cliAlt:"e" usage:"End date for included documents"`
	ScrollSize       int    `cli:"size" usage:"Number of documents that will be returned per shard"`
	Timefield        string `cli:"timefield" usage:"Field name to use for start and end date query"`
	Fieldlist        string `cli:"fields" usage:"Fields to include in export as comma separated list"`
	Fields           []string
}
