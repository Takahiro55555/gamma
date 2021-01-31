# Go パラメータ
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run
GO_ENTRY_POINT_GATEWAY=cmd/manager/main.go
BINARY_DEFAULT_NAME=manager
BINARY_DEFAULT_SUFFIX=out
BINARY_LINUX_NAME=manager-linux
COVERAGE_FILE=cover.out
COVERAGE_FILE_HTML=cover.html
DOCKER_COMPOSE=docker-compose
TEST_DOCKER_COMPOSE_FILE=./build/docker-compose.yml
TEST_DOCKER_COMPOSE_UP_D=$(DOCKER_COMPOSE) -f $(TEST_DOCKER_COMPOSE_FILE) up -d
TEST_DOCKER_COMPOSE_DOWN=$(DOCKER_COMPOSE) -f $(TEST_DOCKER_COMPOSE_FILE) down
level=warn
env=dev
caller= 
host=localhost
port=1883

# Windows, WSL を想定
HTML_OPEN_CMD=explorer.exe
ifeq ($(shell uname),Darwin)
	# MacOS を想定
	HTML_OPEN_CMD=open -a "Safari"
endif

all: test build build-linux-arm64
.PHONY: build  # 擬似ターゲット
build:
	$(GOBUILD) -o $(BINARY_DEFAULT_NAME).$(BINARY_DEFAULT_SUFFIX) $(GO_ENTRY_POINT_GATEWAY)
build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BINARY_LINUX_NAME).arm64 $(GO_ENTRY_POINT_GATEWAY)
.PHONY: test  # 擬似ターゲット
test:
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
coverage:
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) ./...
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_FILE_HTML)
	$(HTML_OPEN_CMD) $(COVERAGE_FILE_HTML)
clean:
	$(GOCLEAN) ./...
	rm -f $(BINARY_DEFAULT_NAME).$(BINARY_DEFAULT_SUFFIX)
	rm -f $(BINARY_LINUX_NAME).arm64
	rm -f $(COVERAGE_FILE) $(COVERAGE_FILE_HTML)
run:
	# $(TEST_DOCKER_COMPOSE_UP_D)
	$(GORUN) cmd/manager/main.go -level ${level} -env ${env} ${caller} -host $(host) -port $(port)
docker:
	$(TEST_DOCKER_COMPOSE_DOWN)
	$(TEST_DOCKER_COMPOSE_UP_D)