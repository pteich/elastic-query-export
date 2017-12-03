# elastic-query-export

Export Data from ElasticSearch to CSV by Raw or Lucene Query (e.g. from Kibana).

## Usage

````bash
elastic-query-export -e "http://localhost:9200" -i "logstash-2017.11.01" --fields="RemoteHost,RequestTime,Timestamp,,RequestUri,RequestProtocol,Agent" -q "RequestUri:*export*"
````

## TODO

Ready to use builds for Windows, Linux, OSX coming soon. More documentation will be added soon too.