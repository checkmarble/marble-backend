FROM golang:1.22 as build

WORKDIR /go/src/app
COPY . .

RUN go get

RUN CGO_ENABLED=0 go build -o /go/bin/app -ldflags="-X 'main.version=`git rev-parse --short HEAD`'"

FROM alpine:3.19

COPY --from=build /go/bin/app /

ENV PORT=${PORT:-8080}
EXPOSE $PORT

ENTRYPOINT ["/app"]