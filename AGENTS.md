# AGENTS.md

Compact instructions for future AI agent sessions working on `devmesh`.

## Architecture Highlights
- **CLI Layer (`internal/cli/`)**: Cobra commands. UI/Styling changes belong here.
- **Proxy Engine (`proxy/`)**: The reverse proxy daemon. Routing logic is in `registry.go` and `proxy.go`. Route targets use `http://localhost:<port>` (never a hardcoded IP) so both IPv4 and IPv6-bound apps are reached.
- **Service Mgmt (`internal/cli/install.go`, `internal/cli/service.go`)**: `devmesh install` (top-level, one-time sudo) installs the CLI to `/usr/local/bin/devmesh` (atomic temp+rename — safe over a running binary, no ETXTBSY) and a systemd unit (Linux) that runs the proxy as the invoking user with `AmbientCapabilities=CAP_NET_BIND_SERVICE` (binds :80 without root). `devmesh proxy start/stop/remove/status` manage the background service; never point `ExecStart` at a project folder (SELinux `user_home_t` vs `bin_t`). Re-running install refreshes the binary (idempotent).
- **Uninstall (`internal/cli/uninstall.go`)**: `devmesh uninstall` is the single full-teardown command (service unit + `/usr/local/bin/devmesh` + `~/.devmesh` with confirmation; `--keep-data` to skip data deletion). There must be only ONE uninstall — proxy-only removal is `proxy remove`.
- **Named project commands (`internal/cli/lifecycle.go`)**: `start/stop/restart [name]` operate on saved state from any directory; without a name they fall back to the cwd flow (`up`/`down`). Named start refuses when the project is already running.
- **System Utilities (`internal/`)**:
  - `daemon_lock.go`: Uses PID-based checks (unreliable in containerized/high-churn envs).
  - `state.go`: Manages project state in `~/.devmesh` ($HOME-based, cross-platform).
- **Process Mgmt (`internal/process/`)**: Manages dev server lifecycles, env injection (`PORT` only when explicitly requested), dynamic port detection (stdout scan + process-group socket poll), and process-group termination.

## Operational Gotchas
- **No sudo, ever**: The CLI must never re-exec itself under sudo. Elevated privileges are needed exactly once, for `sudo devmesh install`. Daily commands (`up`/`start`/`down`/`stop`/`status`/`list`/`restart`/`remove`) run unprivileged; the CLI only spawns unprivileged daemons on `:8080` (the installed service owns `:80`).
- **Port model**: Apps run on the port they intend to use (no `PORT` injection by default); DevMesh detects the bound port and routes it. `ErrAddrInUse` (busy intended port) triggers one restart with an available port injected. Random ports span all unprivileged ports (1024-65535).
- **State Location**: Project state lives in `$HOME/.devmesh` (portable across Linux/macOS/Windows). Do not use system dirs like `/var/lib/devmesh` — they reintroduce root/ownership problems.
- **Windows**: Functionality is currently limited, particularly process liveness checks (`state_windows.go`) and the `service` command (Linux/systemd only; macOS launchd is a planned follow-up).

## Development & Testing
- **Build**: `go build -o bin/devmesh ./cmd/devmesh`
- **Testing**: `go test ./...`
- **Context Awareness**: Many core system functions in `internal/` are context-oblivious. When refactoring system I/O, prioritize injecting `context.Context` for clean teardown.
- **Atomic Operations**: State persistence currently lacks atomic write-then-rename patterns. Use these patterns when refactoring to improve stability.

## Reporting Rule
- **Ticket Implementation Report**: A report of changes, files touched, and any deviations from the ticket must always be given after any ticket implementation.