FROM golang:1.20.1-alpine as build

WORKDIR /go/src/app
COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -o /go/bin/app -ldflags="-X 'main.version=`git describe --tags --abbrev=0`'"

FROM gcr.io/distroless/static

COPY --from=build /go/bin/app /

ENV PORT=8080
EXPOSE $PORT

CMD ["/app"]