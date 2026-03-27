FROM golang:1.23-alpine AS dev

WORKDIR /app

RUN apk add --no-cache ca-certificates git

CMD ["go", "run", "./cmd/api"]

FROM golang:1.23-alpine AS build

WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/rate-limiter ./cmd/api

FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates

COPY --from=build /out/rate-limiter /usr/local/bin/rate-limiter

EXPOSE 8080

CMD ["rate-limiter"]
