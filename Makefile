VERSION ?= dev
BINARY := hello-swarm
LDFLAGS := -s -w -buildid= -X main.version=$(VERSION)

.PHONY: build test check clean docker-build

build:
	CGO_ENABLED=0 go build -mod=readonly -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o "$(BINARY)" .

test:
	go test -mod=readonly ./...

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test -mod=readonly ./...
	CGO_ENABLED=0 go build -mod=readonly -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o /tmp/hello-swarm .

docker-build:
	docker build --build-arg VERSION="$(VERSION)" --tag "hello-swarm:$(VERSION)" .

clean:
	rm -f "$(BINARY)"
