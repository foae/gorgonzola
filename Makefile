all: test install_gorgonzola

install_gorgonzola:
	go install github.com/foae/gorgonzola/cmd/gorgonzola

lint:
	find . -path '*/vendor/*' -prune -o -name '*.go' -type f -exec gofmt -s -w {} \;
	which golangci-lint; \
	if [ $$? -ne 0 ]; then curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
	| sh -s -- -b $(GOPATH)/bin/golangci-lint v1.21.0 \
	chmod +x $(GOPATH)/bin/golangci-lint; \
	fi &&\
	golangci-lint run ./... --disable-all -E errcheck -E bodyclose -E govet -E varcheck -E ineffassign -E gosec -E unconvert -E goconst -E gocyclo -E gofmt -E maligned -E prealloc && go vet ./...

test:
	GOPROXY="off" \
	GOSUMDB="off" \
	go test -v -cover ./...

run: install_gorgonzola
	GOPROXY="off" \
	GOSUMDB="off" \
	$(GOPATH)/bin/gorgonzola
