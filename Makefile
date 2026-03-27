up:
	docker compose up --build api postgres redis

down:
	docker compose down

test:
	docker compose run --rm api go test ./...
