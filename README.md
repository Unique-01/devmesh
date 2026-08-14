# DevMesh CLI

DevMesh is a local development networking CLI that gives your projects stable,
memorable local domains — like `http://ume.local.dev` or `http://vault.local.dev`
— instead of having to remember and type `localhost:<random-port>` for every
microservice you're running.

Under the hood, DevMesh runs a centralized reverse proxy that reads the `Host`
header of incoming requests and forwards them to the right backend port,
combined with automatic `/etc/hosts` entries so the domain actually resolves
on your machine. You start a service once with `devmesh up`, and it's
reachable by name from then on — no more losing track of which port `vault`
was running on.

---

## What it does

- **Named local domains for your services.** `devmesh up --cmd "pnpm dev" --name vault`
  spins up `vault.local.dev`, pointing at whatever port your dev server actually
  bound to.
- **A reverse proxy that auto-starts.** The first `devmesh up` call spawns a
  background proxy daemon if one isn't already running — you never run a
  separate "start the proxy" step.
- **Live route registration.** Every subsequent `devmesh up` (for a different
  service, or a restart) registers its route with the already-running daemon
  over a small local admin API, no daemon restart required.
- **Safe, managed `/etc/hosts` writes.** DevMesh only touches a clearly marked
  block in `/etc/hosts` and cleans up after itself on `devmesh down`.
- **Process lifecycle tracking.** `devmesh status` shows you what's running,
  on what domain, and what PID; `devmesh down` stops it cleanly (whole process
  group, not just the top-level shell).

---

## Permissions

Binding a reverse proxy to port 80 and editing `/etc/hosts` both require root.
You don't need to prefix commands with `sudo` yourself — DevMesh escalates
internally when it needs to (e.g. for `up`/`down`/`restart`/`remove`) and will
prompt you for your password at that point. `status` and `list` are read-only
and never need elevation.

```bash
devmesh up --cmd "pnpm dev" --name vault   # prompts for password if needed
devmesh status                              # no prompt, ever
```

If port 80 isn't available (something else already bound to it, or the
password prompt is declined), DevMesh falls back to `:8080` and tells you —
in that case you'll need `vault.local.dev:8080` in the URL bar, since the
port-free experience specifically depends on being on `:80`.

---

## Installation & building

### Prerequisites
- Go 1.21+
- Linux or macOS (Windows support is present but untested — see Known Limitations)

### Build
```bash
go build -o bin/devmesh ./cmd/devmesh
```

---

## Usage

### 1. Start a service

```bash
devmesh up --cmd "pnpm dev" --name vault
```

This will:
1. Allocate a free local port and inject it as the `PORT` env var for your command.
2. Auto-start the proxy daemon if it isn't already running (tries `:80`, falls back to `:8080`).
3. Register `vault.local.dev -> 127.0.0.1:<allocated-port>` with the daemon.
4. Add the domain to `/etc/hosts`.
5. Run your command in the foreground, streaming its logs directly to your terminal.

Visit `http://vault.local.dev` (or `http://vault.local.dev:8080` if the proxy
fell back off port 80) — no port to remember or type.

### 2. Use a config file instead of flags

Running `devmesh up --cmd "..." --name vault` once writes a `.devmesh.yaml`
in the current directory:

```yaml
name: vault
domain: vault.local.dev
cmd: "pnpm dev"
```

After that, just:
```bash
devmesh up
```

### 3. Check what's running

```bash
devmesh status   # detailed: project, domain, port, PID, status
devmesh list      # just project + domain
```

### 4. Stop a service

```bash
devmesh down
```

Stops the process (whole process group, so child processes spawned by your
dev command — e.g. by `pnpm`/`tsx watch` — are actually terminated), removes
its route from the proxy, and removes its `/etc/hosts` entry.
`.devmesh.yaml` and saved state are kept, so `devmesh up` works again without
re-specifying flags.

### 5. Restart or fully remove

```bash
devmesh restart   # down, then up
devmesh remove    # down, plus deletes .devmesh.yaml and saved state (asks for confirmation)
```

---

## What we built with AO

Built using an Agent Orchestrator (AO) workflow. I planned out the
implementation in phases up front, then handed each phase to an orchestrator
agent, which spawned individual worker agents to build it. I reviewed each
phase's output, then had the orchestrator merge it in — which signals the
worker agent and folds the reviewed work into the codebase.


## Demo

- **Live demo / video:** https://drive.google.com/file/d/1vLKQ_vTInFIsM8La3wAs4xMPSOoNLnel/view?usp=sharing
- **Repo:** https://github.com/unique-01/devmesh 

---

## Project structure

```
cmd/devmesh/            CLI entrypoint (main.go)

internal/cli/            Cobra commands
  lifecycle.go/.test      status, list, down, restart, remove
  up.go/.test              devmesh up
  proxy.go, proxy_launcher.go/.test   devmesh proxy + auto-start/daemon spawning
  root.go/.test, version.go

internal/                 core logic shared across commands
  hosts.go/.test           safe /etc/hosts manager (managed block, sudo fallback)
  identity.go/.test        project name/domain resolution (flags -> config -> cwd)
  state.go, state_unix.go, state_windows.go
                           project state persistence + process-liveness checks
                           (platform-specific: EPERM/ESRCH handling on unix)
  daemon_lock.go           file-lock coordination for concurrent daemon startup
  daemon_wait.go           polling helper for "is the daemon up yet"
  proxy_client.go          HTTP client for the daemon's admin API

internal/process/         process manager: env (PORT) injection, process-group
  manager.go/.test          spawning, graceful termination

proxy/                     the reverse proxy itself
  proxy.go                  Host-header routing / httputil.ReverseProxy handler
  server.go/.test           HTTP server + admin API (/_devmesh/ping, /_devmesh/routes)
  registry.go                route table (domain -> target)
  active_routes.go/.test    live route bookkeeping
  port.go/.test              dynamic port allocation
```

---

## Known limitations

- Windows process-liveness checking is currently broken (`os.Signal(0)` is not
  a valid construction) — Windows build is present but not functional for
  status checks. Not prioritized for this submission; flagged here rather than
  left silent.
- The proxy daemon's admin API (`/_devmesh/routes`) is unauthenticated on
  `127.0.0.1` — acceptable for a local dev tool, not something to expose
  beyond localhost.
- Ports are reallocated randomly on every `devmesh up`, not held sticky per
  project across restarts.

---

## Running tests

```bash
go test ./...
```
