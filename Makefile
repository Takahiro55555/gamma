# Go パラメータ
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run
GO_ENTRY_POINT_GATEWAY=cmd/gateway/main.go
BINARY_NAME=gateway.out
COVERAGE_FILE=cover.out
COVERAGE_FILE_HTML=cover.html

HTML_OPEN_CMD=explorer.exe  # Windows, WSL を想定
ifeq ($(shell uname),Darwin)
	# MacOS を想定
	HTML_OPEN_CMD=open -a "Safari"
endif

all: test build
.PHONY: build  # 擬似ターゲット
build:
	$(GOBUILD) -o $(BINARY_NAME) $(GO_ENTRY_POINT_GATEWAY)
test:
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
coverage:
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_FILE_HTML)
	$(HTML_OPEN_CMD) $(COVERAGE_FILE_HTML)
clean:
	$(GOCLEAN) ./...
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE) $(COVERAGE_FILE_HTML)
run:
	$(GORUN) cmd/gateway/main.go
