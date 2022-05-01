FROM golang:1.18-alpine as build
WORKDIR /
COPY . ./

RUN apk add \
    build-base \
    git \
&&  go build -ldflags="-s -w"

FROM alpine
ENV GIN_MODE=release

COPY --from=build /bitcoin-balance-notifier /
COPY web /web
VOLUME [ "/db" ]

CMD ["/bitcoin-balance-notifier"]
