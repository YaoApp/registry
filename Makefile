GO ?= go
GIT ?= git
GOFMT ?= gofmt "-s"
GOFILES := $(shell find . -name "*.go" -not -path "./vendor/*")
PACKAGES ?= $(shell $(GO) list ./...)
VERSION := $(shell grep 'var Version' handlers/info.go | awk '{print $$4}' | sed 's/"//g')
COMMIT := $(shell $(GIT) log --format='%h' -n 1 2>/dev/null || echo "unknown")
NOW := $(shell date +"%FT%T%z")
LDFLAGS := -X github.com/yaoapp/registry/handlers.Version=$(VERSION)

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GOFILES)

.PHONY: fmt-check
fmt-check:
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: vet
vet:
	$(GO) vet $(PACKAGES)

.PHONY: unit-test
unit-test:
	echo "mode: count" > coverage.out
	for d in $(PACKAGES); do \
		$(GO) test -v -covermode=count -coverprofile=profile.out $$d > tmp.out; \
		cat tmp.out; \
		if grep -q "^--- FAIL" tmp.out; then \
			rm tmp.out; \
			exit 1; \
		elif grep -q "build failed" tmp.out; then \
			rm tmp.out; \
			exit 1; \
		elif grep -q "runtime error" tmp.out; then \
			rm tmp.out; \
			exit 1; \
		fi; \
		if [ -f profile.out ]; then \
			cat profile.out | grep -v "mode:" >> coverage.out; \
			rm profile.out; \
		fi; \
	done

.PHONY: e2e-test
e2e-test:
	$(GO) test -v -count=1 ./e2e/...

.PHONY: test
test: vet unit-test e2e-test

.PHONY: build
build: clean
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o dist/registry .
	chmod +x dist/registry

.PHONY: build-all
build-all: clean
	mkdir -p dist/release
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w $(LDFLAGS)" -o dist/release/registry-$(VERSION)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-s -w $(LDFLAGS)" -o dist/release/registry-$(VERSION)-linux-arm64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-s -w $(LDFLAGS)" -o dist/release/registry-$(VERSION)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-s -w $(LDFLAGS)" -o dist/release/registry-$(VERSION)-darwin-arm64 .
	chmod +x dist/release/*
	ls -lh dist/release/

.PHONY: clean
clean:
	rm -rf dist
	rm -f coverage.out profile.out tmp.out
