FROM golang:1.25-bookworm AS build

ARG MARBLE_VERSION=dev
ARG SEGMENT_WRITE_KEY=

WORKDIR /go/src/app

RUN apt update && apt install -y libgeos-dev libgeos3.11.1

COPY go.mod go.sum /go/src/app/
RUN go mod download -x

COPY . .

RUN curl https://cdn.checkmarble.com/ip-database/marble.mmdb.gz | gzip -d > infra/default-ipdb.mmdb
RUN CGO_ENABLED=1 go build -o /go/bin/app -trimpath -ldflags="-extldflags=-s -w -X main.apiVersion=${MARBLE_VERSION} -X main.segmentWriteKey=${SEGMENT_WRITE_KEY}"

FROM gcr.io/distroless/cc:latest

COPY --from=build /go/bin/app /
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
COPY --from=build /usr/lib/x86_64-linux-gnu/libgeos* /usr/lib/x86_64-linux-gnu/

ENV ZONEINFO=/zoneinfo.zip
ENV PORT=${PORT:-8080}

EXPOSE $PORT

ENTRYPOINT ["/app"]
