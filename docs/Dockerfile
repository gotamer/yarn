# Builder for `static` tool
FROM golang:alpine AS static

ENV CGO_ENABLED=0

RUN go install git.mills.io/prologic/static@latest

# Builder for the Jekyll site
FROM jekyll/builder AS jekyll

RUN mkdir -p /srv/jekyll

WORKDIR /srv/jekyll

COPY . .

RUN jekyll build

# Runtime
FROM scratch

WORKDIR /
VOLUME /data

COPY --from=static /go/bin/static /static
COPY --from=jekyll /srv/jekyll/_site /data

ENTRYPOINT ["/static"]
CMD ["-r", "/data"]
