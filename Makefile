all: lint test install_gorgonzola

install_gorgonzola:
	go install github.com/foae/gorgonzola/cmd/gorgonzola

lint:
	find . -path '*/vendor/*' -prune -o -name '*.go' -type f -exec gofmt -s -w {} \;
	which gometalinter; if [ $$? -ne 0 ]; then go get -u github.com/alecthomas/gometalinter && gometalinter --install; fi
	gometalinter --vendor --exclude=repos --disable-all --enable=golint ./...
	go vet ./...

test:
	go test -v -short -cover ./...

run: install_gorgonzola
	HTTP_LISTEN_ADDR="127.0.0.1:8000" \
	DNS_LISTEN_PORT="53" \
	UPSTREAM_DNS_SERVER_IP="1.1.1.1" \
	ENV="dev" \
	$(GOPATH)/bin/gorgonzola
