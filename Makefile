# PromptOps - AI Model Backend Switcher
BINARY=promptops
VERSION=2.5.0

# Build directories
BUILD_DIR=build

# Go build flags
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION} -X main.buildVersion=${VERSION}"

.PHONY: all clean build linux macos macos-arm install

all: clean build

clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY)

build:
	go build $(LDFLAGS) -o $(BINARY) .

linux:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 .

macos:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 .

macos-arm:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 .

release: clean linux macos macos-arm
	@echo "Built binaries in $(BUILD_DIR)/"

install: build
	cp $(BINARY) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(BINARY)"

test:
	go test -v ./...

fmt:
	go fmt .
