# elastic-query-export

Export Data from ElasticSearch to CSV by Raw or Lucene Query (e.g. from Kibana).
Works with ElasticSearch 5.x and makes use of ElasticSearch's Scroll API and Go's
concurrency possibilities to work fast.


## Usage

````bash
elastic-query-export -e "http://localhost:9200" -i "logstash-2017.11.*" --fields="RemoteHost,RequestTime,Timestamp,,RequestUri,RequestProtocol,Agent" -q "RequestUri:*export*"
````

## TODO

Ready to use builds for Windows, Linux, OSX coming soon. More documentation will be added soon too.
