# Contributing to Distributed Cache Library

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful and professional in all interactions.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/distributed-cache.git`
3. Create a feature branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Run tests: `go test ./...`
6. Commit your changes: `git commit -am 'Add your feature'`
7. Push to your fork: `git push origin feature/your-feature`
8. Create a Pull Request

## Development Setup

### Prerequisites

- Go 1.25 or later
- Redis server (for testing)
- Docker (optional, for Redis in container)

### Running Redis for Development

```bash
# Using Docker
docker run -d -p 6379:6379 redis:latest

# Or install locally
brew install redis
redis-server
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestName ./...
```

## Code Style

- Follow Go conventions and best practices
- Use `gofumpt` for formatting: `gofumpt -w -extra ./`
- Use `golint` for linting: `golint ./...`
- Write clear, descriptive commit messages
- Add comments for exported functions and types
