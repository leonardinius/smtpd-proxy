# Borrowed from:
# https://gist.github.com/turtlemonvh/38bd3d73e61769767c35931d8c70ccb4
BINARY		= smtpd-proxy
APPMODULE	= app
TEST_REPORT	= test-report.log
LINT_REPORT	= report-lint.xml
BUILDDIR	= $(shell pwd)
BUILDOUT	= $(shell pwd)/bin

COMMIT		= $(shell git rev-parse --short HEAD)
BRANCH		= $(shell git rev-parse --abbrev-ref HEAD)

GOPATH		?= ${shell go env GOPATH}
GOOS		?= $(shell go env GOHOSTOS)
GOARCH		?= $(shell go env GOHOSTARCH)

# go source files, ignore vendor directory
GOFILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS = -ldflags "-X github.com/leonardinius/smtpd-proxy/app/cmd.COMMIT=${COMMIT} -X github.com/leonardinius/smtpd-proxy/app/cmd.BRANCH=${BRANCH} -s -w"

# Build the project
all: clean test lint build

bin: linux windows darwin

gorun:
	cd ${BUILDDIR}; \
	GOOS=${GOOS} GOARCH=${GOARCH} go run ./${APPMODULE} --verbose --configuration=smtpd-proxy.yml;

run: build
	cd ${BUILDDIR}; \
    ${BUILDOUT}/${BINARY}-${GOOS}-${GOARCH} --configuration=smtpd-proxy.yml;

build: $(GOFILES)
	cd ${BUILDDIR}; mkdir -p ${BUILDOUT};\
	GOOS=${GOOS} GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}/${BINARY}-${GOOS}-${GOARCH} ./${APPMODULE};

linux: $(GOFILES)
	cd ${BUILDDIR}; mkdir -p ${BUILDOUT}; \
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}/${BINARY}-linux-${GOARCH} ./${APPMODULE};

darwin: $(GOFILES)
	cd ${BUILDDIR}; mkdir -p ${BUILDOUT}; \
	GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}/${BINARY}-darwin-${GOARCH} ./${APPMODULE} ;

windows: $(GOFILES)
	cd ${BUILDDIR}; mkdir -p ${BUILDOUT}; \
	GOOS=windows GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILDOUT}/${BINARY}-windows-${GOARCH}.exe ./${APPMODULE} ;

test: $(GOFILES)
	cd ${BUILDDIR}; \
	go test -race -timeout=120s -count 1 -parallel 4 -v ./... 2>&1

ci-test: $(GOFILES)
	cd ${BUILDDIR}; \
	go install github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt@latest; \
	go test -race -timeout=120s -count 1 -parallel 4 -v ./... -json 2>&1 | tee ${BUILDOUT}/${TEST_REPORT} | gotestfmt

lint: $(GOFILES)
	-cd ${BUILDDIR}; \
	mkdir -p ${BUILDOUT}; \
    if ! hash golangci-lint 2>/dev/null; then \
	  curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH} v1.48.0; \
      go get -u github.com/mattn/goveralls; \
	fi; \
	golangci-lint run --config .golangci.yml --out-format=junit-xml ./... > ${BUILDOUT}/${LINT_REPORT} 2>&1;

fmt: $(GOFILES)
	cd ${BUILDDIR}; \
	go fmt $$(go list ./... | grep -v /vendor/) ;

clean: $(GOFILES)
	-rm -f ${BUILDOUT}/${LINT_REPORT}
	-rm -f ${BUILDOUT}/${TEST_REPORT}
	-rm -f ${BUILDOUT}/${BINARY}-*

.PHONY: gorun clean test
# .PHONY: gorun run build linux darwin windows test lint fmt clean
