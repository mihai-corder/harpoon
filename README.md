# harpoon 🎯

> Terminal-first API testing. **Collections as code**: debug in the terminal, smoke-test in CI, share the report.

**harpoon** defines API request collections as plain YAML files that live in your repo — versioned, reviewable, diffable. Run them from the terminal (CLI or TUI), in the browser, or as CI smoke tests — always the *same files*. Chain variables between requests (login → token → authenticated call). No lock-in, no proprietary format.

![CI](https://github.com/mihai-corder/harpoon/actions/workflows/ci.yml/badge.svg)

<!-- TODO(v0.3): replace with a recorded GIF of the TUI.
![demo](docs/demo.gif)
-->

📖 **完整中文用户手册**：[docs/guide.zh-CN.md](docs/guide.zh-CN.md) — 从零开始的安装、配置参考、实战菜谱与排错 FAQ

## Why harpoon?

Existing tools solve half the workflow. GUI-era clients (Postman/newman) lock collections into a proprietary format; terminal clients (posting, slumber, ATAC) stop at interactive debugging; CI-first tools (hurl) invent their own file format and have no interactive UX. harpoon is one tool for the whole loop:

| | harpoon | hurl | posting / slumber | newman |
|---|---|---|---|---|
| Collections as plain YAML, git-friendly | ✅ | own format | ✅ | Postman format |
| Interactive terminal UX | ✅ TUI | ❌ | ✅ | ❌ |
| CI smoke tests from the same files | ✅ | ✅ | ❌ | ✅ (Postman-locked) |
| PR report bot | ✅ | ❌ | ❌ | ❌ |
| Shareable web report | ✅ | ❌ | ❌ | ❌ |

## Install

```sh
go install github.com/mihai-corder/harpoon@latest
```

Requires Go 1.22+ to build. A single static binary, no runtime dependencies. Prebuilt binaries for Windows / Linux / macOS are available on the [Releases](https://github.com/mihai-corder/harpoon/releases) page.

## Quickstart

Run the bundled demo against GitHub's public API (no auth needed):

```sh
harpoon run testdata/example/collection.yaml
```

```
  NAME        METHOD  URL                                                        STATUS  TIME     RESULT
  zen         GET     https://api.github.com/zen                                 200     218ms    ok
  user-info   GET     https://api.github.com/users/octocat                       200     156ms    ok
  first-repo  GET     https://api.github.com/users/octocat/repos?per_page=1...   200     194ms    ok

  3 passed, 0 failed in 572ms
```

Write your own collection:

```yaml
# collection.yaml
requests:
  login:
    method: POST
    url: "{{base_url}}/auth/login"
    body:
      username: demo
      password: "{{password}}"
    extract:
      token: "{{response.body.token}}"

  list-users:
    method: GET
    url: "{{base_url}}/users?page=1"
    headers:
      Authorization: "Bearer {{token}}"   # ← chained from login's extract
    expect:
      status: 200
      contains: "users"
      max_ms: 800
```

```sh
harpoon run collection.yaml --env dev --var password=s3cret
```

## Commands

| Command | Purpose |
|---|---|
| `harpoon run <collection>` | Execute requests in order, print summary. Exit code 1 on any failure — CI-friendly. |
| `harpoon ci <collection>` | CI variant: JSON + HTML reports, same exit-code contract. |
| `harpoon share <report.html>` | Upload a report, get a public link (expires in 90 days, revocable). |
| `harpoon tui <collection>` | Interactive terminal UI: pick requests, run them, inspect responses. |
| `harpoon serve <collection>` | Local web UI in your browser: click to run, inspect, browse history. |
| `harpoon validate <collection>` | Structural + reference checks, no HTTP. Undefined variables are warnings. |
| `harpoon list <collection>` | List requests in run order. |

## Interactive TUI

```sh
harpoon tui collection.yaml
```

A two-pane interface: pick requests on the left, inspect responses on the right — status, per-assertion results, extracted variables, headers and pretty-printed JSON bodies. The session carries extracted variables across runs: log in once, then re-run any authenticated request on its own.

| Key | Action |
|---|---|
| `↑/k` `↓/j` | Move in the focused pane |
| `tab` | Switch focus between request list and response |
| `enter` | Run the selected request |
| `R` | Run all requests, in collection order |
| `h` | Session history (newest first) |
| `ctrl+u` / `ctrl+d` | Scroll the response half a page |
| `q` | Quit |

## Web UI

```sh
harpoon serve collection.yaml            # opens http://localhost:8080
harpoon serve collection.yaml --addr :3000 --open=false
```

A dark, mouse-first interface served from inside the binary: click requests to run them, inspect status badges, per-assertion results, extracted variables, headers and syntax-highlighted JSON, and browse session history in a slide-over drawer. The frontend is plain HTML/CSS/JS embedded with `//go:embed` — the tool stays a single static executable with zero runtime dependencies, and the same runner session powers it, so variable chains work here too.

## CI / GitHub Action

`harpoon ci` writes machine-readable reports and keeps the same exit-code contract as `run`:

```sh
harpoon ci smoke.yaml --json report.json --html report.html
```

The official composite action in this repository wires it into any workflow — it installs harpoon, runs your collection, uploads the HTML report as an artifact and posts a results table as a PR comment:

```yaml
name: Smoke Test
on: [push]

permissions:
  pull-requests: write   # needed for the PR comment

jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: mihai-corder/harpoon@main   # pin a tag after release
        with:
          collection: smoke.yaml
          # env: dev
          # vars: |
          #   base_url=${{ vars.STAGING_URL }}
          token: ${{ github.token }}
```

The PR comment shows a per-request table with pass/fail and the exact assertion failures, plus a link to the shared HTML report. On push branches the run behaves identically minus the comment.

## Report sharing

```sh
harpoon ci smoke.yaml --html report.html
harpoon share report.html
```

```text
  shared:  https://harpoon-share.mihai-corder.workers.dev/r/Ab3xK9mQ
  expires: 2027-02-19T00:00:00Z
  revoke:  curl -X DELETE "https://.../r/Ab3xK9mQ" -H "X-Delete-Token: ..."
```

Reports live on a tiny Cloudflare Worker ([server/](server/README.md)): 90-day expiry, 1 MiB cap, 20 uploads/hour per IP, and served with `Content-Security-Policy: sandbox` so uploaded content can never run scripts on the origin. Anyone can self-host the Worker and point `--server` at their own instance.

Useful flags on `run`:

| Flag | Effect |
|---|---|
| `--env dev` / `-e dev` | Load `harpoon.dev.yaml` (a plain key-value file) from the collection's directory |
| `--var k=v` / `-V k=v` | Override a variable; repeatable, highest priority |
| `--request login,list-users` / `-r` | Run a subset, in collection order |

## Configuration reference

**Requests** (all templated fields accept variables):

| Field | Meaning |
|---|---|
| `method` | HTTP method; defaults to `GET` |
| `url` | Target URL |
| `headers` | Map of headers |
| `body` | Arbitrary YAML value, rendered and sent as JSON |
| `vars` | Request-scoped variables; may reference outer variables |
| `extract` | Variables to pull from the response, visible to later requests |
| `expect.status` | Exact expected status code |
| `expect.contains` | Substring the body must include |
| `expect.max_ms` | Max allowed duration in ms |

A request without `expect` passes as long as the HTTP exchange succeeds.

**Syntax**:

| Pattern | Resolved from |
|---|---|
| `{{name}}` | Variables: CLI `--var` > extracted values > request `vars` > profile > collection `vars` |
| `${env:NAME}` | Process environment — secrets never touch disk |
| `{{response.status}}` | Extract: the response's status code |
| `{{response.headers.Name}}` | Extract: a response header (case-insensitive) |
| `{{response.body.a.b.0}}` | Extract: a JSON body path (objects by key, arrays by index) |

## Roadmap

- [x] **v0.1** — core engine: YAML collections, variable chaining, assertions, summary output
- [x] **v0.2** — TUI: interactive runner, response inspector, session-chained runs, history
- [x] **v0.3** — local web UI: `harpoon serve`, embedded frontend, JSON API, session history
- [x] **v0.4** — CI: `harpoon ci` (JSON + HTML reports) + official GitHub Action with PR comment bot
- [x] **v0.5** — report sharing: `harpoon share` + Cloudflare Worker host, public links in PR comments

*The original roadmap is complete — harpoon is now in maintenance mode.*

## Development

```sh
go test ./...     # unit + httptest integration tests
go vet ./...
gofmt -l .        # CI enforces formatting
```

## License

[MIT](LICENSE)
