# DevMesh CLI

DevMesh is a local development networking CLI that gives your projects stable,
memorable local domains — like `http://ume.localhost` or `http://vault.localhost`
— instead of having to remember and type `localhost:<random-port>` for every
microservice you're running.

Under the hood, DevMesh runs a centralized reverse proxy that reads the `Host`
header of incoming requests and forwards them to the right backend port,
leveraging built-in `.localhost` resolution supported natively by all modern operating systems. You start a service once with `devmesh up`, and it's
reachable by name from then on — no more losing track of which port `vault`
was running on.

---

## What it does

- **Named local domains for your services.** `devmesh up --cmd "pnpm dev" --name vault`
  spins up `vault.localhost`, pointing at whatever port your dev server actually
  bound to.
- **A reverse proxy that auto-starts.** The first `devmesh up` call spawns a
  background proxy daemon if one isn't already running — you never run a
  separate "start the proxy" step.
- **Live route registration.** Every subsequent `devmesh up` (for a different
  service, or a restart) registers its route with the already-running daemon
  over a small local admin API, no daemon restart required.
- **Native `.localhost` resolution.** No `/etc/hosts` editing or root privileges required for domain resolution.
- **An always-on proxy service (optional).** `sudo devmesh install` once sets up a
  systemd service that binds `:80` and starts on boot — after that, no command ever
  needs elevated privileges.
- **Run projects from anywhere.** `devmesh start vault` starts a previously-run
  project using its saved metadata — no need to `cd` into the project folder;
  `stop`/`restart` accept a name the same way.
- **Process lifecycle tracking.** `devmesh status` shows you what's running,
  on what domain, and what PID; `devmesh down` stops it cleanly (whole process
  group, not just the top-level shell).

---

## Permissions

DevMesh never re-execs itself under `sudo`. The only operation that needs root
is binding the proxy to port 80, and that is handled **once** at install time:

```bash
sudo devmesh install   # one-time: systemd unit, proxy runs as YOUR user
```

Install receives only the `CAP_NET_BIND_SERVICE` capability (the minimal
privilege needed to bind `:80`), runs the proxy as your normal user with your
`$HOME`, and auto-starts on boot with restart-on-crash. It also copies the
binary to `/usr/local/bin/devmesh` (systemd/SELinux refuse to execute binaries
from home directories) so `devmesh` is callable from anywhere. After that,
`up`, `start`, `down`, `stop`, `status`, `list`, `restart`, `remove` all run
without sudo. Re-run `sudo devmesh install` after rebuilding to refresh the
service binary.

If the service isn't installed, `devmesh up` still works: it auto-spawns an
unprivileged proxy daemon on `:8080` and reminds you about `devmesh install` —
in that case URLs need the port suffix, e.g. `vault.localhost:8080`.

```bash
devmesh install        # sudo once: CLI to /usr/local/bin + always-on proxy on :80
devmesh proxy start    # sudo: start the background proxy service
devmesh proxy stop     # sudo: stop the background proxy service (unit kept)
devmesh proxy remove   # sudo: remove the background proxy service (CLI + data kept)
devmesh proxy status   # works without sudo
```

### Uninstalling

There is a single uninstall command that removes everything:

```bash
devmesh uninstall            # sudo: service + /usr/local/bin/devmesh + ~/.devmesh
```

Running it without sudo removes only your saved state (with a confirmation
prompt) and prints the exact `sudo devmesh uninstall` command for the
root-owned parts. Flags/behavior:

- `--keep-data` — skip deleting `~/.devmesh` (no prompt).
- `devmesh proxy remove` — remove **only** the background proxy service,
  keeping the CLI and data.
- Project `.devmesh.yaml` files are never touched; a `~/go/bin/devmesh` copy
  from `go install` is managed by the Go toolchain, not by DevMesh.

---

## Installation & building

### Install (release binary)

1. Download the artifact for your platform from GitHub Releases
   (e.g. `devmesh-linux-amd64.tar.gz`).
2. Extract and make it executable:
   ```bash
   tar -xzf devmesh-linux-amd64.tar.gz
   chmod +x devmesh
   ```
3. Install the always-on proxy (one-time sudo; also installs the CLI to
   `/usr/local/bin` so `devmesh` is callable from anywhere):
   ```bash
   sudo ./devmesh service install
   ```

Go developers can also use `go install` for a development copy — note it
installs to `~/go/bin` only and does **not** install the proxy service.

### Build from source
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
1. Auto-start the proxy daemon if it isn't already running (tries `:80`, falls back to `:8080`).
2. Let your command run on the port it intends to use (e.g. Vite on `5173`) — DevMesh
   does not inject `PORT` by default. The bound port is discovered automatically
   (from the app's startup output, or by inspecting its listening sockets) and the
   route `vault.localhost -> localhost:<detected-port>` is registered with the daemon
   (forwarded via `localhost` so both IPv4 and IPv6-bound apps are reached).
3. If the app's intended port is already taken by another program and the app
   crashes on it, DevMesh restarts it once with an available port injected as the
   `PORT` env var (pass `--port` to pin a specific port instead).
4. Run your command in the foreground, streaming its logs directly to your terminal.

Visit `http://vault.localhost` (or `http://vault.localhost:8080` if the proxy
fell back off port 80) — no port to remember or type.

### 2. Use a config file instead of flags

Running `devmesh up --cmd "..." --name vault` once writes a `.devmesh.yaml`
in the current directory:

```yaml
name: vault
domain: vault.localhost
cmd: "pnpm dev"
```

After that, just:
```bash
devmesh up
```

### 3. Start a project from anywhere

Once a project has been run at least once, start it by name from any
directory — no need to `cd` into the project folder:

```bash
devmesh start vault      # uses saved metadata (directory, command, identity)
devmesh stop vault       # stop it from anywhere
devmesh restart vault    # stop + start
```

`start`/`stop`/`restart` without a name operate on the current directory's
project, just like `up`/`down`.

### 4. Check what's running

```bash
devmesh status   # detailed: project, domain, port, PID, status
devmesh list      # just project + domain
```

### 5. Stop a service

```bash
devmesh down
```

Stops the process (whole process group, so child processes spawned by your
dev command — e.g. by `pnpm`/`tsx watch` — are actually terminated), and removes
its route from the proxy.
`.devmesh.yaml` and saved state are kept, so `devmesh up` works again without
re-specifying flags.

### 6. Restart or fully remove

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
  service.go               proxy service lifecycle subcommands (start/stop/remove/status)
  install.go/.test         devmesh install (CLI + systemd service, one-time sudo)
  uninstall.go/.test       devmesh uninstall (single full teardown)
  lifecycle.go/.test       status, list, start/stop/restart (named or cwd), down, remove
  root.go/.test, version.go

internal/                 core logic shared across commands
  identity.go/.test        project name/domain resolution (flags -> config -> cwd)
  state.go, state_unix.go, state_windows.go
                           project state persistence (~/.devmesh, $HOME-based)
                           + process-liveness checks
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
- The `service` command targets Linux/systemd only for now; macOS (launchd)
  and Windows service support are planned follow-ups.

---

## Running tests

```bash
go test ./...
```
