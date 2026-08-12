# DevMesh CLI

DevMesh is a local development networking CLI that gives projects stable local domains while automatically managing their underlying ports.

## Project Structure

- `cmd/devmesh/main.go`: Entry point of the application.
- `internal/cli/`: Cobra CLI commands (root, version, etc.) and tests.
- `pkg/`: Public library code (if needed).

## Getting Started

### Prerequisites

- Go 1.21 or later installed.

### Build

```bash
go build -o bin/devmesh ./cmd/devmesh
```

### Run

```bash
./bin/devmesh --help
./bin/devmesh version
```

### Run Tests

```bash
go test ./...
```
