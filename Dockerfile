FROM alpine:latest
LABEL org.opencontainers.image.source https://github.com/pteich/elastic-query-export

COPY dist/elastic-query-export_linux_amd64/elastic-query-export /usr/bin/elastic-query-export

ENTRYPOINT ["/usr/bin/elastic-query-export"]