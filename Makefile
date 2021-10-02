-include environ.inc
.PHONY: deps dev build install image release test clean

CGO_ENABLED=0
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
	  minify -b -o ./internal/static/css/twtxt.min.css ./internal/static/css/[0-9]*-*.css;	\
	  minify -b -o ./internal/static/js/twtxt.min.js ./internal/static/js/[0-9]*-*.js;		\
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

test:
	@$(GOCMD) test -v -cover -race ./...

bench: bench-twtxt.txt
	go test -race -benchtime=1x -cpu 16 -benchmem -bench "^(Benchmark)" git.mills.io/yarnsocial/yarn/types

bench-twtxt.txt:
	curl -s https://twtxt.net/user/prologic/twtxt.txt > $@

clean:
	@git clean -f -d -X
