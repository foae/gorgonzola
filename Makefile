all: lint test install_gorgonzola

install_gorgonzola:
	go install github.com/foae/gorgonzola/cmd/gorgonzola

lint:
	find . -path '*/vendor/*' -prune -o -name '*.go' -type f -exec gofmt -s -w {} \;
	which golangci-lint; if [ $$? -ne 0 ]; then wget -O - -q https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.21.0; fi
	golangci-lint run ./... --disable-all -E errcheck -E bodyclose -E govet -E varcheck -E ineffassign -E gosec -E unconvert -E goconst -E gocyclo -E gofmt -E maligned -E prealloc

test:
	go test -v -short -cover ./...

run: install_gorgonzola
	$(GOPATH)/bin/gorgonzola
