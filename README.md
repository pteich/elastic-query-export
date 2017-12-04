# elastic-query-export

Export Data from ElasticSearch to CSV by Raw or Lucene Query (e.g. from Kibana).
Works with ElasticSearch 5.x and makes use of ElasticSearch's Scroll API and Go's
concurrency possibilities to work fast.

## Install

Download a pre-compiled binary for your operating system from here: https://github.com/pteich/elastic-query-export/releases
You need just this binary. It works on OSX (Darwin), Linux and Windows.

## Usage

````bash
es-query-export -e "http://localhost:9200" -i "logstash-2017.11.*" --fields="RemoteHost,RequestTime,Timestamp,RequestUri,RequestProtocol,Agent" -q "RequestUri:*export*"
````

## CLI Options

| Flag         | Default               |                | 
|--------------|-----------------------|----------------|
| `-h --help`    |                       | show help      |
| `-v --version` |                       | show version   |
| `-e --eshost`  | http://localhost:9200 | URI to ElasticSearch instance  | 
| `-i --index`   | logs-*                | name of index to use, you can use globbing characters |
| `-q --query`   |                       | Lucene query to match documents (same as in Kibana) |
| `-f --field`   | _all                  | limit export to specific field(s) add as many `-f` as you need |
| `   --fields`  |                       | define a comma separated list of fields to export (overrides `-f`) |
| `-o --outfile` | output.csv            | name of output file |
| `-r --rawquery`|                       | optional raw ElasticSearch query JSON string |

## TODO

Right now you can only export time ranges by index name. To make this more fine grained there need to be added flags for start and stop time. 
