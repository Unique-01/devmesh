<a id="readme-top"></a>

[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]

<br />
<div align="center">
  <h3 align="center">DevMesh</h3>
  <p align="center">
    Give your local services a name, not a port.
    <br />
    <a href="https://drive.google.com/file/d/1vLKQ_vTInFIsM8La3wAs4xMPSOoNLnel/view?usp=sharing">Watch the demo</a>
    &middot;
    <a href="https://github.com/Unique-01/devmesh/issues/new?labels=bug">Report a bug</a>
    &middot;
    <a href="https://github.com/Unique-01/devmesh/issues/new?labels=enhancement">Request a feature</a>
  </p>
</div>

---

Stop memorizing which port belongs to which project.

```text
localhost:5173   localhost:3000   localhost:8787
        ↓                ↓                ↓
frontend.localhost   api.localhost   vault.localhost
```

```sh
devmesh up --cmd "pnpm dev" --name vault
```

That's it — `vault.localhost` now points at whatever port `pnpm dev` picked. DevMesh starts your process, detects its port, and wires up the routing. Your app never has to know or care.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Install

**With Go:**

```sh
go install github.com/Unique-01/devmesh@latest
```

**From source:**

```sh
git clone https://github.com/Unique-01/devmesh.git
cd devmesh
go build -o bin/devmesh ./cmd/devmesh
```

Requires Go 1.26+ to build. On Linux, `lsof` improves port detection (`sudo apt install lsof` / `sudo dnf install lsof`); it's preinstalled on macOS.

No proxy running yet? DevMesh spins up an unprivileged one on `:8080` automatically — no sudo, works immediately. Want clean port-80 URLs with no `:8080` suffix and an always-on proxy that survives reboots? Run `sudo devmesh install` once.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Usage

**First run** — save the config so you don't retype flags:

```sh
devmesh up --cmd "pnpm dev" --name vault
```

This writes a `.devmesh.yaml` to the project folder. Every run after that is just:

```sh
devmesh up
```

**Manage projects by name, from anywhere:**

```sh
devmesh start vault
devmesh stop vault      # kills the process, frees the route, keeps the config
devmesh restart vault
devmesh remove vault    # stop + delete the config
```

**See what's running:**

```sh
devmesh ps               # every DevMesh project
devmesh status            # is the proxy up, how many routes are active
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## How it works

`.localhost` domains already resolve to your machine on modern OSes, so DevMesh doesn't touch `/etc/hosts` or run a DNS server — it's just a reverse proxy. When a project starts, DevMesh detects the port it bound and registers the route; when it stops, the route is removed. Commands run in their own process group, so `stop`/`restart` kill the whole tree, not just the parent.

DevMesh never runs as root. The only privileged step is the one-time `sudo devmesh install`, which lets the proxy bind port 80 — the proxy itself still runs as your user. Skip it entirely and everything works through the unprivileged `:8080` fallback.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Roadmap

- [x] Reverse proxy with live route registration
- [x] Automatic port detection
- [x] Process-group lifecycle management
- [x] Unprivileged `:8080` fallback + optional port-80 install
- [ ] macOS `launchd` service support
- [ ] Windows service support + native port detection
- [ ] Wildcard routes (`*.localhost`)
- [ ] Optional auth for the proxy admin API

See [open issues][issues-url] for details.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Contributing

PRs and issues welcome.

```sh
git checkout -b feature/AmazingFeature
git commit -m 'Add some AmazingFeature'
git push origin feature/AmazingFeature
```

Run `go test ./...` before submitting.

<a href="https://github.com/Unique-01/devmesh/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Unique-01/devmesh" alt="contrib.rocks image" />
</a>

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## License

MIT — see [`LICENSE`](LICENSE).

## Contact

[Unique-01][github-profile-url] &middot; [github.com/Unique-01/devmesh][project-url]

Built with [spf13/cobra][cobra-url]. README structure inspired by [Best-README-Template][best-readme-template].

<!-- MARKDOWN LINKS & IMAGES -->
[contributors-shield]: https://img.shields.io/github/contributors/Unique-01/devmesh.svg?style=for-the-badge
[contributors-url]: https://github.com/Unique-01/devmesh/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/Unique-01/devmesh.svg?style=for-the-badge
[forks-url]: https://github.com/Unique-01/devmesh/network/members
[stars-shield]: https://img.shields.io/github/stars/Unique-01/devmesh.svg?style=for-the-badge
[stars-url]: https://github.com/Unique-01/devmesh/stargazers
[issues-shield]: https://img.shields.io/github/issues/Unique-01/devmesh.svg?style=for-the-badge
[issues-url]: https://github.com/Unique-01/devmesh/issues
[license-shield]: https://img.shields.io/github/license/Unique-01/devmesh.svg?style=for-the-badge
[license-url]: https://github.com/Unique-01/devmesh/blob/main/LICENSE
[github-profile-url]: https://github.com/Unique-01
[project-url]: https://github.com/Unique-01/devmesh
[best-readme-template]: https://github.com/othneildrew/Best-README-Template
[cobra-url]: https://github.com/spf13/cobra