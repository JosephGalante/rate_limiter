.PHONY: up up-detached down migrate-up migrate-down sqlc test demo-bootstrap

up:
	docker compose up -d postgres redis
	docker compose run --rm migrate
	docker compose up --build api web

up-detached:
	docker compose up -d postgres redis
	docker compose run --rm migrate
	docker compose up -d --build api web

down:
	docker compose down

migrate-up:
	docker compose run --rm migrate

migrate-down:
	docker compose run --rm migrate down 1

sqlc:
	docker compose run --rm sqlc

test:
	docker compose run --rm api go test ./...

demo-bootstrap:
	docker compose run --rm api go run ./cmd/demo-bootstrap
