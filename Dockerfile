FROM golang:1.27.3-trixie AS build

ARG MARBLE_VERSION=dev
ARG SEGMENT_WRITE_KEY=

WORKDIR /go/src/app

COPY go.mod go.sum /go/src/app/

RUN \
  --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg/mod \
  go mod download -x

COPY . .

RUN curl https://cdn.checkmarble.com/ip-database/marble.mmdb.gz | gzip -d > infra/default-ipdb.mmdb

RUN \
  --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg/mod \
  CGO_ENABLED=1 go build -o /go/bin/app -trimpath -ldflags="-extldflags=-s -w -X main.apiVersion=${MARBLE_VERSION} -X main.segmentWriteKey=${SEGMENT_WRITE_KEY}"

FROM gcr.io/distroless/cc-debian13 AS runtime

ENV ZONEINFO=/zoneinfo.zip
ENV PORT=8080

EXPOSE $PORT

ENTRYPOINT ["/app"]

# Used by CI, from a context containing a binary and zoneinfo.zip built on the same distribution as the build stage.
FROM runtime AS prebuilt

COPY --chmod=755 app /
COPY zoneinfo.zip /

FROM runtime

COPY --from=build /go/bin/app /
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
