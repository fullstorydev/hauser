dev_build_version=$(shell git describe --tags --always --dirty)
# to_check is all code in this repo that we want to run checks on
# (it is all Go code in here, but intentionally excludes the
# vendor folder contents)
dirs_to_check=./ ./warehouse ./pipeline ./config
files_to_check=$(shell find . -maxdepth 1 -mindepth 1 -type f -name '*.go')
all_to_check=$(files_to_check) $(dirs_to_check)

# TODO: run golint and errcheck, but only to catch *new* violations and
# decide whether to change code or not (e.g. we need to be able to whitelist
# violations already in the code). They can be useful to catch errors, but
# they are just too noisy to be a requirement for a CI -- we don't even *want*
# to fix some of the things they consider to be violations.
.PHONY: ci
ci: deps checkgofmt vet staticcheck ineffassign predeclared test

.PHONY: deps
deps:
	go get -d -v -t ./...

.PHONY: updatedeps
updatedeps:
	go get -d -v -t -u -f ./...

.PHONY: install
install:
	go install -ldflags '-X "main.version=dev build $(dev_build_version)"' .

.PHONY: checkgofmt
checkgofmt:
	gofmt -s -l $(all_to_check)
	@if [ -n "$$(gofmt -s -l $(all_to_check))" ]; then \
		exit 1; \
	fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: staticcheck
staticcheck:
	@go get honnef.co/go/tools/cmd/staticcheck
	staticcheck ./...

.PHONY: ineffassign
ineffassign:
	@go get github.com/gordonklaus/ineffassign
	ineffassign $(all_to_check)

.PHONY: predeclared
predeclared:
	@go get github.com/nishanths/predeclared
	predeclared $(all_to_check)

# Intentionally omitted from CI, but target here for ad-hoc reports.
.PHONY: golint
golint:
	@go get golang.org/x/lint/golint
	golint -min_confidence 0.9 -set_exit_status . $(dirs_to_check)

# Intentionally omitted from CI, but target here for ad-hoc reports.
.PHONY: errcheck
errcheck:
	@go get github.com/kisielk/errcheck
	errcheck ./...

.PHONY: test
test:
	go test -race . $(dirs_to_check)
