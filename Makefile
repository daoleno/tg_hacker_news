.PHONY: build run clean docker-build docker-run docker-stop test

# Variables
BINARY_NAME=tg-hacker-news
DOCKER_IMAGE=tg-hacker-news

# Build the Go binary
build:
	CGO_ENABLED=1 go build -o $(BINARY_NAME) main.go

# Run the application locally
run: build
	./$(BINARY_NAME)

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f *.db

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker services
docker-stop:
	docker-compose down

# View logs
logs:
	docker-compose logs -f

# Run tests (if any)
test:
	go test -v ./...

# Install dependencies
deps:
	go mod download
	go mod tidy

# Create data directory
init:
	mkdir -p data

# Development setup
dev-setup: deps init
	@echo "Development environment ready!"
	@echo "1. Copy .env.example to .env"
	@echo "2. Edit .env with your bot token"
	@echo "3. Run 'make run' to start the bot"

# Full Docker setup
docker-setup: docker-build
	@echo "Docker setup complete!"
	@echo "1. Copy .env.example to .env"
	@echo "2. Edit .env with your bot token"
	@echo "3. Run 'make docker-run' to start the bot"