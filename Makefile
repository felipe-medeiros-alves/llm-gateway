.PHONY: up down test proto smoke integration mock

up:
	docker compose up -d --build

down:
	docker compose down -v

mock:
	go run ./scripts/mockserver

test:
	go test ./... -count=1

integration:
	go test ./internal/cache/ -count=1 -tags=integration -run TestSemantic

proto:
	protoc --go_out=. --go-grpc_out=. --go_opt=module=github.com/meta/llm-gateway --go-grpc_opt=module=github.com/meta/llm-gateway proto/gateway/v1/gateway.proto

smoke:
	bash scripts/smoke_e2e.sh
