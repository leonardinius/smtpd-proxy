# Borrowed from: 
# https://gist.github.com/turtlemonvh/38bd3d73e61769767c35931d8c70ccb4
BINARY		= smtpd-proxy
APPMODULE	= app
TEST_REPORT	= report-tests.xml
LINT_REPORT	= report-lint.xml
BUILDDIR	= $(shell pwd)
BUILDOUT	= $(shell pwd)/bin

VERSION		?= 0.0.1
COMMIT		= $(shell git rev-parse --short HEAD)
BRANCH		= $(shell git rev-parse --abbrev-ref HEAD)

GOPATH		= ${shell go env GOPATH}
GOOS		= $(shell go env GOHOSTOS)
GOARCH		= $(shell go env GOHOSTARCH)

# go source files, ignore vendor directory
GOFILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS = -ldflags "-X main.VERSION=${VERSION} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH} -s -w"

# Build the project
all: clean test lint build

gorun: 
	cd ${BUILDDIR}; \
	GOOS=${GOOS} GOARCH=${GOARCH} go run ./${APPMODULE} --verbose --configuration=smtpd-proxy.yml; \
	cd - >/dev/null

run: build
	cd ${BUILDDIR}; \
    ${BUILDOUT}/${BINARY}-${GOOS}-${GOARCH} --configuration=smtpd-proxy.yml; \
	cd - >/dev/null

build: $(GOFILES)
	cd ${BUILDDIR}; \
	GOOS=${GOOS} GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}/${BINARY}-${GOOS}-${GOARCH} ./${APPMODULE}; \
	cd - >/dev/null

linux: $(GOFILES)
	cd ${BUILDDIR}; \
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}/${BINARY}-linux-${GOARCH} ./${APPMODULE}; \
	cd - >/dev/null

darwin: $(GOFILES)
	cd ${BUILDDIR}; \
	GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}/${BINARY}-darwin-${GOARCH} ./${APPMODULE} ; \
	cd - >/dev/null

windows: $(GOFILES)
	cd ${BUILDDIR}; \
	GOOS=windows GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}//${BINARY}-windows-${GOARCH}.exe ./${APPMODULE} ; \
	cd - >/dev/null

test: $(GOFILES)
	cd ${BUILDDIR}; \
	go test -timeout=60s -parallel 4 -v ./... 2>&1; \
	cd - >/dev/null

citest: $(GOFILES)
	if ! hash go2xunit 2>/dev/null; then go get -u github.com/tebeka/go2xunit; fi
	cd ${BUILDDIR}; \
	go test -race -timeout=60s -count 1 -parallel 4 -v ./... 2>&1 | go2xunit -output ${BUILDOUT}/${TEST_REPORT} ; \
	cd - >/dev/null

lint: $(GOFILES)
	-cd ${BUILDDIR}; \
    if ! hash golangci-lint 2>/dev/null; then \
	  curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH} v1.48.0; \
      go get -u github.com/mattn/goveralls; \
	fi; \
	golangci-lint run --config .golangci.yml --out-format=junit-xml ./... > ${BUILDOUT}/${LINT_REPORT} 2>&1; \
	cd - >/dev/null

fmt: $(GOFILES)
	cd ${BUILDDIR}; \
	go fmt $$(go list ./... | grep -v /vendor/) ; \
	cd - >/dev/null

clean: $(GOFILES)
	-rm -f ${BUILDOUT}/${TEST_REPORT}
	-rm -f ${BUILDOUT}/${TEST_REPORT}.tmp
	-rm -f ${BUILDOUT}/${LINT_REPORT}
	-rm -f ${BUILDOUT}/${BINARY}-*

.PHONY: gorun clean test
# .PHONY: gorun run build linux darwin windows test lint fmt clean
