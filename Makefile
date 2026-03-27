up:
	docker compose up --build api postgres redis

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
