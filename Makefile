all: lint test install_gorgonzola

install_gorgonzola:
	go install github.com/foae/gorgonzola/cmd/gorgonzola

lint:
	which golangci-lint; if [ $$? -ne 0 ]; then go get -u github.com/golangci/golangci-lint; fi
	golangci-lint run ./... --disable-all -E errcheck -E bodyclose -E govet -E varcheck -E ineffassign -E gosec -E unconvert -E goconst -E gocyclo -E gofmt -E maligned -E prealloc
	go vet ./...

test:
	go test -v -short -cover ./...

test-coverage:
	go test ./... -coverpkg=./... -coverprofile cover.out.tmp && \
	cat cover.out.tmp | grep -v "mock.go" | grep -v "generated.go" | grep -v "_gen.go" > cover.out && \
    go tool cover -func cover.out

run: install_gorgonzola
	HTTP_LISTEN_ADDR="127.0.0.1:8000" \
	DNS_LISTEN_PORT="53" \
	UPSTREAM_DNS_SERVER_IP="116.203.111.0" \
	ENV="dev" \
	$(GOPATH)/bin/gorgonzola
