# harpoon 🎯

> Terminal-first API testing. **Collections as code**: debug in the terminal, smoke-test in CI, share the report.

**harpoon** defines API request collections as plain YAML files that live in your repo — versioned, reviewable, diffable. Run them interactively from the terminal, chain variables between requests (login → token → authenticated call), and feed the *exact same files* into CI as smoke tests. No lock-in, no proprietary format, no GUI required.

![CI](https://github.com/mihai-corder/harpoon/actions/workflows/ci.yml/badge.svg)

<!-- TODO(v0.3): replace with a recorded GIF of the TUI.
![demo](docs/demo.gif)
-->

## Why harpoon?

Existing tools solve half the workflow. GUI-era clients (Postman/newman) lock collections into a proprietary format; terminal clients (posting, slumber, ATAC) stop at interactive debugging; CI-first tools (hurl) invent their own file format and have no interactive UX. harpoon is one tool for the whole loop:

| | harpoon | hurl | posting / slumber | newman |
|---|---|---|---|---|
| Collections as plain YAML, git-friendly | ✅ | own format | ✅ | Postman format |
| Interactive terminal UX | ✅ TUI | ❌ | ✅ | ❌ |
| CI smoke tests from the same files | ✅ | ✅ | ❌ | ✅ (Postman-locked) |
| PR report bot + shareable web report | 🔜 v0.3–0.4 | ❌ | ❌ | ❌ |

## Install

```sh
go install github.com/mihai-corder/harpoon@latest
```

Requires Go 1.22+ to build. A single static binary, no runtime dependencies.

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
| `harpoon tui <collection>` | Interactive terminal UI: pick requests, run them, inspect responses. |
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
- [ ] **v0.3** — `harpoon ci` + official GitHub Action: PR comment bot, HTML report
- [ ] **v0.4** — shareable web reports (report upload, public links)

## Development

```sh
go test ./...     # unit + httptest integration tests
go vet ./...
gofmt -l .        # CI enforces formatting
```

## License

[MIT](LICENSE)
