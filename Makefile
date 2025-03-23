.PHONY: build run clean test lint migrate

# Variables
APP_NAME = notification-service
MAIN_PATH = ./cmd/server
MIGRATION_DIR = ./migrations

# Construcción y ejecución
build:
	go build -o $(APP_NAME) $(MAIN_PATH)

run:
	go run $(MAIN_PATH)

clean:
	rm -f $(APP_NAME)
	go clean

# Pruebas
test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Linting
lint:
	golangci-lint run

# Migraciones
migrate-up:
	migrate -path $(MIGRATION_DIR) -database "postgres://postgres:postgres@localhost:5432/rantipaydb?sslmode=disable" up

migrate-down:
	migrate -path $(MIGRATION_DIR) -database "postgres://postgres:postgres@localhost:5432/rantipaydb?sslmode=disable" down

migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir $(MIGRATION_DIR) -seq $$name

# Docker
docker-build:
	docker build -t $(APP_NAME) .

docker-run:
	docker run -p 8080:8080 -p 9090:9090 $(APP_NAME)

docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

docker-compose-logs:
	docker-compose logs -f

# Generación de protobuf
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		./pkg/proto/*.proto

# Instalación de herramientas
install-tools:
	go install github.com/golang/protobuf/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest