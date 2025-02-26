FROM golang:1.23 as build

ARG MARBLE_VERSION=dev

WORKDIR /go/src/app
COPY . .

RUN go get

RUN CGO_ENABLED=0 go build -o /go/bin/app -ldflags="-X 'main.apiVersion=${MARBLE_VERSION}'"

FROM alpine:3.19

COPY --from=build /go/bin/app /
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
ENV ZONEINFO=/zoneinfo.zip

ENV PORT=${PORT:-8080}
EXPOSE $PORT

ENTRYPOINT ["/app"]