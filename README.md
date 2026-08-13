# DevMesh CLI

DevMesh is a powerful local development networking and proxy CLI that gives your projects stable local domains (such as `http://ume.local.dev` or `http://vault.local.dev`) without requiring you to manually type port numbers (e.g. `:35443`). DevMesh runs a centralized reverse proxy on a fixed local port (defaulting to `:8080`, or `:80`/`:443` with privilege) along with automatic domain resolution via `/etc/hosts` + sudo, dynamic port allocation, and automated process lifecycle management.

---

## Features

- **Port-Free Domain Routing**: Access services directly at `http://ume.local.dev` without typing port numbers. DevMesh's proxy daemon auto-starts and routes requests to the correct backend service port.
- **Stable Local Domains**: Automatic domain generation (e.g., `<project>.local.dev`) or custom domains.
- **Automatic Port Management**: Assigns and injects the `PORT` environment variable to your development commands.
- **Zero-Config `/etc/hosts` Setup**: Automatically updates and cleans up `/etc/hosts` entries securely.
- **Auto-Daemonization**: The proxy daemon is automatically spawned as a background process when you run `devmesh up`.

---

## Installation & Building

### Prerequisites

- Go 1.21 or later installed.
- Linux / macOS / Windows.

### Build

```bash
go build -o bin/devmesh ./cmd/devmesh
```

---

## Usage Guide

### 1. Start the DevMesh Proxy Daemon
DevMesh now automatically starts the reverse proxy daemon when you run `devmesh up`. 

If you prefer to start it manually:

```bash
devmesh proxy
```

*Note: DevMesh tries to bind to port 80. If permission is denied, it will automatically fall back to port 8080.*


### 2. Run Your Development Service (`devmesh up`)

In your project directory, start your application with `devmesh up`. DevMesh automatically allocates an available port, injects the `PORT` environment variable, configures the local domain in `/etc/hosts`, and registers the route with the proxy daemon.

```bash
# Example: Start a web app on port-free domain ume.local.dev
devmesh up --cmd "python3 -m http.server $PORT" --name ume
```

Alternatively, if you create a `.devmesh.yaml` in your repository root:

```yaml
name: ume
domain: ume.local.dev
cmd: "python3 -m http.server $PORT"
```

You can simply run:

```bash
devmesh up
```

Now, visiting **`http://ume.local.dev:8080`** (or directly on `:80`/`:443` if bound to standard HTTP ports) routes seamlessly to your running application without manual port juggling!

### 3. Check Active Services (`devmesh status`)
View all currently running DevMesh-managed services, their assigned ports, PIDs, and domains:

```bash
devmesh status
```


### 4. Stop Services (`devmesh down`)

Stop a running project service and clean up its active routes and `/etc/hosts` entries:

```bash
devmesh down
```

---

## Project Structure

- `cmd/devmesh/`: CLI entrypoint (`main.go`).
- `internal/cli/`: Cobra commands (`up`, `proxy`, `ps`, `down`, `version`, etc.).
- `internal/hosts.go`: Safe `/etc/hosts` manager with sudo support.
- `proxy/`: HTTP reverse proxy server and route registry.
- `internal/process/`: Process manager with environment port injection and lifecycle tracking.

---

## Running Tests

```bash
go test ./...
```
