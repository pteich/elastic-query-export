BINARY=es-query-csv
VERSION=1.0.3
BUILD_TIME=`date +%FT%T%z`
GOX_OSARCH="darwin/amd64 linux/386 linux/amd64 windows/386 windows/amd64"

default: build

clean:
	rm -rf ./bin

build:
	CGO_ENABLED=0 \
	go build -a -o ./bin/${BINARY}-${VERSION} *.go

build-linux:
	CGO_ENABLED=0 \
	GOARCH=amd64 \
	GOOS=linux \
	go build -ldflags "-X main.Version=${VERSION}" -a -o ./bin/${BINARY}-${VERSION} *.go

build-gox:
	gox -ldflags "-X main.Version=${VERSION}" -osarch=${GOX_OSARCH} -output="bin/${VERSION}/{{.Dir}}_{{.OS}}_{{.Arch}}"

deps:
	dep ensure;

test:
	go test
