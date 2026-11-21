.PHONY: all run build test vet fmt fmt-check tidy comments check clean

all: build

run:
	go run ./cmd/bot

build:
	mkdir -p bin
	go build -o bin/bot ./cmd/bot

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || { \
		echo "Go files need formatting; run 'make fmt'."; exit 1; \
	}

comments:
	go run ./tools/commentlint

tidy:
	go mod tidy

check: fmt-check comments test vet

clean:
	rm -rf bin coverage.out
