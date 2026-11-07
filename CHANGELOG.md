# Changelog

## [0.2.0] - 2026-11-07

### Breaking

- See repository diff for API changes in this release (secure defaults, renamed packages where applicable, context-aware constructors).

### Security

- Hardened defaults: TLS, timeouts, body limits, PII redaction where logging applies.

### Fixed

- Bug fixes from the v0.2.0 hardening pass.

### Changed

- CI uses go-version-file; added race, staticcheck, govulncheck, golangci-lint (gosec).


All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-05-19

### Added

- HTTP client with logging `RoundTripper` (`NewClient`, `NewTransport`, `NewDebugTransport`)
- Default metadata-only logging (no request/response body buffering)
- Optional body logging with truncation (`WithMaxBodyLogBytes`, `DefaultMaxBodyLogBytes`)
- Sensitive query/header redaction before log emit
- Integration with [go-logging](https://github.com/andywang2011/go-logging) via `request.Context()`

### Removed

- `pkg/logging` wrapper (callers initialize `go-logging` in the host application)
- `context.go` encrypt-status bridge (use global go-logging mask config)

[0.1.0]: https://github.com/andywang2011/go-httplib/releases/tag/v0.1.0
