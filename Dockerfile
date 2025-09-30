FROM golang:1.25-bookworm AS build

ARG MARBLE_VERSION=dev
ARG SEGMENT_WRITE_KEY=

WORKDIR /go/src/app

COPY go.mod go.sum /go/src/app/
RUN go mod download -x

COPY . .

RUN CGO_ENABLED=1 go build -o /go/bin/app -trimpath -ldflags='-extldflags=-s -w -X main.apiVersion="${MARBLE_VERSION}" -X main.segmentWriteKey="${SEGMENT_WRITE_KEY}"'

FROM gcr.io/distroless/cc:latest

COPY --from=build /go/bin/app /
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
ENV ZONEINFO=/zoneinfo.zip

ENV PORT=${PORT:-8080}
EXPOSE $PORT

ENTRYPOINT ["/app"]
