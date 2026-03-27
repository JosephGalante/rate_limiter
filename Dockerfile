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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/demo-bootstrap ./cmd/demo-bootstrap
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate-db ./cmd/migrate
RUN chmod +x ./scripts/start-render.sh

FROM alpine:3.21 AS runtime

RUN apk add --no-cache ca-certificates

COPY --from=build /out/rate-limiter /usr/local/bin/rate-limiter
COPY --from=build /out/demo-bootstrap /usr/local/bin/demo-bootstrap
COPY --from=build /out/migrate-db /usr/local/bin/migrate-db
COPY --from=build /app/scripts/start-render.sh /usr/local/bin/start-render

EXPOSE 8080

CMD ["rate-limiter"]
