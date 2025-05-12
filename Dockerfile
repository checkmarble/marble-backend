FROM golang:1.24 AS build

ARG MARBLE_VERSION=dev
ARG SEGMENT_WRITE_KEY=

WORKDIR /go/src/app
COPY . .

RUN go get

RUN CGO_ENABLED=0 go build -o /go/bin/app -ldflags="-X 'main.apiVersion=${MARBLE_VERSION}' -X 'main.segmentWriteKey=${SEGMENT_WRITE_KEY}'"

FROM alpine:3.19

COPY --from=build /go/bin/app /
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
ENV ZONEINFO=/zoneinfo.zip

ENV PORT=${PORT:-8080}
EXPOSE $PORT

ENTRYPOINT ["/app"]