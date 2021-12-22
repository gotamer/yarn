-include environ.inc
.PHONY: deps dev build install image release test clean

export CGO_ENABLED=0
VERSION=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "$VERSION")
COMMIT=$(shell git rev-parse --short HEAD || echo "$COMMIT")
GOCMD=go
GOVER=$(shell go version | grep -o -E 'go1\.17\.[0-9]+')

all: preflight build

preflight:
	@./preflight.sh

deps:
	@$(GOCMD) install github.com/tdewolff/minify/v2/cmd/minify@latest
	@$(GOCMD) install github.com/nicksnyder/go-i18n/v2/goi18n@latest
	@$(GOCMD) install github.com/astaxie/bat@latest

dev : DEBUG=1
dev : build
	@./yarnc -v
	@./yarnd -D -O -R $(FLAGS)

cli:
	@$(GOCMD) build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w \
		-X $(shell go list).Version=$(VERSION) \
		-X $(shell go list).Commit=$(COMMIT)" \
		./cmd/yarnc/...

server: generate
	@$(GOCMD) build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w \
		-X $(shell go list).Version=$(VERSION) \
		-X $(shell go list).Commit=$(COMMIT)" \
		./cmd/yarnd/...

build: cli server

generate:
	@if [ x"$(DEBUG)" = x"1"  ]; then		\
	  echo 'Running in debug mode...';	\
	else								\
	  minify -b -o ./internal/theme/static/css/yarn.min.css ./internal/theme/static/css/[0-9]*-*.css;	\
	  minify -b -o ./internal/theme/static/js/yarn.min.js ./internal/theme/static/js/[0-9]*-*.js;		\
	fi

install: build
	@$(GOCMD) install ./cmd/yarnc/...
	@$(GOCMD) install ./cmd/yarnd/...

ifeq ($(PUBLISH), 1)
image:
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t prologic/yarnd .
	@docker push prologic/yarnd
else
image:
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t prologic/yarnd .
endif

release:
	@./tools/release.sh

fmt:
	@$(GOCMD) fmt ./...

test:
	@$(GOCMD) test -v -cover -race ./...

coverage:
	@$(GOCMD) test -v -cover -race -cover -coverprofile=coverage.out  ./...
	@$(GOCMD) tool cover -html=coverage.out

bench: bench-yarn.txt
	go test -race -benchtime=1x -cpu 16 -benchmem -bench "^(Benchmark)" git.mills.io/yarnsocial/yarn/types

bench-yarn.txt:
	curl -s https://twtxt.net/user/prologic/twtxt.txt > $@

clean:
	@git clean -f -d -X
