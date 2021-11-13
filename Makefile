# Go パラメータ
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

BINARY_DEFAULT_SUFFIX=out

COVERAGE_FILE=cover.out
COVERAGE_FILE_HTML=cover.html

# Windows, WSL を想定
HTML_OPEN_CMD=explorer.exe
ifeq ($(shell uname),Darwin)
	# MacOS を想定
	HTML_OPEN_CMD=open -a "Safari"
endif

all: test build
.PHONY: build  # 擬似ターゲット
build: clean
	./build.sh manager
	./build.sh gateway

.PHONY: test  # 擬似ターゲット
test:
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
coverage:
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_FILE_HTML)
	$(HTML_OPEN_CMD) $(COVERAGE_FILE_HTML)
clean:
	$(GOCLEAN) ./...
	rm -f *.$(BINARY_DEFAULT_SUFFIX)
	rm -f *.arm64
	rm -f *.arm
	rm -f $(COVERAGE_FILE) $(COVERAGE_FILE_HTML)
run:
	./run.sh run
stop:
	./run.sh
