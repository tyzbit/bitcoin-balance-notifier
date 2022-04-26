FROM golang:1.18-alpine as build
WORKDIR /
COPY . ./

RUN apk add \
    build-base \
    git \
&&  go build -ldflags="-s -w"

FROM alpine

COPY --from=build /bitcoin-balance-notifier /
VOLUME [ "/db" ]

CMD ["/bitcoin-balance-notifier"]
