# Security Policy

## Status of this policy — no warranties, no obligations

`terraform-provider-nile` is free software under the [European Union Public
Licence 1.2](LICENSE) and is provided **"AS IS"**, without warranties or
conditions of any kind, express or implied — including, but not limited to,
fitness for a particular purpose, merchantability, or the absence of defects.

This document is **informational only**. Nothing in it constitutes a
contractual obligation, service-level agreement, support commitment, or
guarantee of any kind, neither towards users nor towards vulnerability
reporters. In particular, there is no obligation to:

- acknowledge, respond to, or investigate any report,
- fix, mitigate, or publicly disclose any issue, or to do so within any
  timeframe,
- issue security advisories, patched releases, credits, or bounties,
- maintain or support any particular version.

Any handling of reports and fixes occurs exclusively on a voluntary,
best-effort basis, as time and resources permit, and may be declined or
discontinued at any time. If any part of this policy could be read as
conflicting with the licence, the licence prevails.

This policy also grants no authorization to test any third-party system,
including the Nile service — see the scope section below.

## Scope

This policy is intended to cover the `terraform-provider-nile` repository
([`Golden-Apple-Research/terraform-provider-nile`](https://github.com/Golden-Apple-Research/terraform-provider-nile)).

In scope:

- Vulnerabilities in the provider's Go code, including the API client
  (`internal/nileapi`), the resource and data source implementations, and the
  provider schema.
- Weaknesses in how the provider handles secrets — API tokens, database
  credentials, invite codes — including leaks into logs, plans or state
  beyond what is documented.
- Weaknesses in the build, test and release pipeline (CI workflows,
  `Makefile` targets, `dev_overrides` / plugin mirror instructions).

Out of scope — please refer such issues to the respective vendor instead:

- Vulnerabilities in the [Nile](https://www.thenile.dev) service or its API —
  this project is an independent client and has no control over the service.
- Vulnerabilities in Terraform core or the Terraform Registry (HashiCorp).
- Vulnerabilities in third-party Go dependencies. Report them upstream; if a
  flaw in a dependency is exploitable specifically through this provider,
  you may coordinate with us, but we do not commit to anything.

## Security-relevant behaviour (for context)

When assessing a report, note that some security-relevant behaviour is
intentional and documented in the [`README.md`](README.md) behaviour notes:

- Terraform state contains sensitive values (credential passwords, invite
  codes, `raw_json` payloads). This is inherent to Terraform's design; users
  are responsible for keeping state in an encrypted backend with restricted
  access. Secrets are marked sensitive and common secret fields in `raw_json`
  are redacted to `[REDACTED]` before they reach state.
- The provider only talks to the API over HTTPS (plain HTTP is limited to
  loopback test endpoints) and never follows redirects, so the bearer token
  cannot be forwarded elsewhere.
- Non-idempotent requests are never retried automatically.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub
issues, discussions, or pull requests.**

Instead, use one of these private channels:

1. **GitHub private vulnerability reporting** (preferred):
   [Report a vulnerability](https://github.com/Golden-Apple-Research/terraform-provider-nile/security/advisories/new)
2. **Email**: [malaccoda@top-email.net](mailto:malaccoda@top-email.net)

Please include as much of the following as you can:

- A description of the issue and its impact.
- Steps to reproduce or a proof of concept.
- The affected version (release tag) or commit hash.
- Any output, logs or configuration needed to trigger the issue.

### What to expect

Reports are read and triaged on a voluntary, best-effort basis. You may
receive an acknowledgement, follow-up questions, or a notification about a
fix — none of which is promised. If a fix is published, it will generally
appear in a new release of the provider; a GitHub Security Advisory and/or
reporter credit may be included at our sole discretion. As a courtesy, we
ask for a reasonable disclosure window before any public disclosure, but
this request itself creates no obligation on either side.

Good-faith research that respects user data and service availability is
appreciated.

## Version handling

The project is pre-1.0 and maintained voluntarily. Fixes, when they are
made, are generally applied to the latest release line and to `main`.
Earlier releases are not maintained, and there is no commitment to maintain
any release at all.
