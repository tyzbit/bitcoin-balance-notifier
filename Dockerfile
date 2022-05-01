FROM golang:1.18-alpine as build
WORKDIR /app/
COPY . /app/

RUN apk add \
    build-base \
    git \
&&  go build -ldflags="-s -w"

FROM alpine
ENV GIN_MODE=release

COPY --from=build /app/bitcoin-balance-notifier /
VOLUME [ "/db" ]

CMD ["/bitcoin-balance-notifier"]
