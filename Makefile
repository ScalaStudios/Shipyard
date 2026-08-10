.PHONY: tidy test server runner web typecheck compose-up compose-down

tidy:
	go mod tidy

test:
	go test ./...

server:
	go run ./cmd/shipyard-server

runner:
	go run ./cmd/shipyard-runner

web:
	yarn workspace @shipyard/web dev

typecheck:
	yarn typecheck

compose-up:
	docker compose -f deploy/compose/compose.yml up --build

compose-down:
	docker compose -f deploy/compose/compose.yml down
