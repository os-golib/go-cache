#!/bin/bash

set -e

echo "Setting up development environment..."

# Install development tools
echo "Installing development tools..."
make dev-tools

# Download dependencies
echo "Downloading dependencies..."
make deps

# Start development services
echo "Starting development services..."
docker compose -f docker-compose.yml up -d

# Wait for services to be healthy
echo "Waiting for services to be ready..."
sleep 10

# Run initial linting and tests
echo "Running initial checks..."
make lint
make test

echo "Development environment setup complete!"
echo "Services:"
echo "  - PostgreSQL: localhost:5432"
echo "  - Redis: localhost:6379"
echo "  - Redis Commander: localhost:8081"
echo "  - MySQL: localhost:3306"