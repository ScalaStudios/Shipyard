.PHONY: tidy test server web typecheck compose-up compose-down

tidy:
	go mod tidy

test:
	go test ./...

server:
	go run ./cmd/shipyard-server

web:
	yarn workspace @shipyard/web dev

typecheck:
	yarn typecheck

compose-up:
	docker compose -f deploy/compose/compose.yml up --build

compose-down:
	docker compose -f deploy/compose/compose.yml down
