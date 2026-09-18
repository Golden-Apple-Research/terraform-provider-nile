# Nile API contract tooling

Nile publishes an official OpenAPI 3.0.1 spec for its control-plane REST API:

- <https://global.thenile.dev/openapi.json>
- <https://global.thenile.dev/openapi.yaml>

The spec is pinned into this repository (`tests/spec/nile-openapi.json` plus
`tests/spec/VERSION`) so that tests are deterministic and CI can detect when
Nile changes its API contract.

## What lives here

| File | Purpose |
| --- | --- |
| `../spec/nile-openapi.json` | Pinned Nile API spec (generated artifact, do not hand-edit) |
| `../spec/VERSION` | Pinned spec version, e.g. `0.1.0-74222e6` (includes the upstream git SHA) |
| `fetch-spec.sh` | Download the live spec and re-pin it |
| `check-drift.sh` | Fail when the pinned spec and the live spec diverge (version or content) |
| `run-prism-proxy.sh` | Stateful mock API + [Prism](https://github.com/stoplightio/prism) validation proxy in front of it |
| `run-schemathesis.sh` | [Schemathesis](https://schemathesis.readthedocs.io) property-based contract fuzzing against the mock |
| `check-schemathesis-baseline.py` | Classify Schemathesis deviations against the known baseline |
| `schemathesis-baseline.txt` | Accepted deviations (operation + check heading), regenerated consciously |

## How the pieces fit together

```
                       tests/spec/nile-openapi.json (pinned, committed)
                            |                        |
        validates every request+response        generates fuzz cases
                            v                        v
  provider/mock  --->  Prism proxy :18081  --->  mock API :18080   <---  Schemathesis
                       (warn-only proxy)         (stateful: READY
                       tests/api/run-prism-      polling, 409, retry)
                       proxy.sh                  tests/api/run-schemathesis.sh
```

Three independent drift detectors:

1. **Prism validation proxy** (`make api-prism`) — forwards everything to the
   stateful mock but logs any request or response that deviates from the
   pinned spec. Watch the log while developing against the mock.
2. **Smoke test through the proxy** (`make smoke-prism`) — the full end-to-end
   smoke run, with the same live spec validation on every exchange.
3. **Schemathesis** (`make api-schemathesis`) — generates property-based test
   cases from the spec and fails with a minimal reproducing case when the
   mock's responses do not conform to the documented contract.

### Known-deviations baseline

Schemathesis currently finds deviations between the mock and Nile's spec
(e.g. the mock answers `404` for unknown databases on routes where the spec
documents no `404`, and hostnames like `foo.db.thenile.dev` violate the
spec's `format: uri` for `dbHost`). These are recorded in
`schemathesis-baseline.txt`, and only NEW deviations fail the run. When a new
deviation is intentional (the real API changed, or the mock was adjusted),
refresh the baseline deliberately:

```sh
ST_REPORT_DIR=/tmp/st-out tests/api/run-schemathesis.sh .venv/bin/st
.venv/bin/python tests/api/check-schemathesis-baseline.py \
  /tmp/st-out/st-report.xml tests/api/schemathesis-baseline.txt --update
```

and review the diff before committing. Harness errors (connection refused,
unparseable spec) are never baselined — they always fail.

`make api-drift-check` (and the scheduled `API drift` GitHub workflow) catches
upstream changes: Nile's spec version embeds the upstream git SHA, so any
API change bumps it. On drift, refresh with `make api-spec-fetch`, review
`git diff tests/spec/` for contract changes, adapt provider/mock if needed,
and commit the refreshed pin.

## Requirements

- `bash`, `curl`, `python3` (mock API + scripts)
- `node`/`npx` (Prism is fetched via `npx @stoplight/prism-cli@5`)
- `schemathesis` for the fuzzing script, e.g.
  `python3 -m venv .venv && .venv/bin/pip install 'schemathesis==3.38.*' 'hypothesis<6.113'`
  (hypothesis is pinned because newer releases break schemathesis 3.38.x;
  CI installs the same pins, see `.github/workflows/api-drift.yml`)

## Manual usage

```sh
make api-spec-fetch     # re-pin the current live spec
make api-drift-check    # pinned vs. live (exit 1 on drift)
make api-prism          # mock + Prism proxy until Ctrl-C
make smoke-prism        # full smoke test through the validation proxy
make api-schemathesis   # contract fuzzing against the mock
```
