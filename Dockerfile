FROM golang:1.26.4 AS build
WORKDIR /opt/api

COPY go.mod go.sum ./
RUN go mod download

COPY ./internal ./internal
COPY ./cmd ./cmd

RUN CGO_ENABLED=0 go build -o main ./cmd

FROM alpine:3.24.2
WORKDIR /opt/api

RUN adduser -D -H app

COPY --from=build --chown=app:app /opt/api/main /opt/api/main

USER app
ENTRYPOINT [ "./main" ]
