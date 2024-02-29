FROM golang:1.22 as build

WORKDIR /go/src/app
COPY . .

RUN go get

RUN CGO_ENABLED=0 go build -o /go/bin/app -ldflags="-X 'main.version=`git rev-parse --short HEAD`'"

FROM gcr.io/distroless/static

COPY --from=build /go/bin/app /

ENV PORT=8080
EXPOSE $PORT

ENTRYPOINT ["/app"]