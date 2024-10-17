FROM alpine:latest

COPY elastic-query-export /usr/bin/elastic-query-export

ENTRYPOINT ["/usr/bin/elastic-query-export"]