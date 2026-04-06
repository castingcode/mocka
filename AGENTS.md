# Mocka — Agent Context

This file provides context for AI coding assistants working in this repository.
Read it fully before making any changes.

## What This Project Is

Mocka is a Go library that provides a mock MOCA server for use in integration tests.
Its primary use is as an imported library — consuming projects embed it to stand up a
fake MOCA endpoint rather than depending on a real BlueYonder WMS server.
The binary in `cmd/mockasrv` is a convenience wrapper; it is not the primary artifact.

MOCA (Middleware Objects, Commands, and Adapters) is a scripting/command language used
by BlueYonder warehouse management systems. MOCA commands are plain-text strings that
follow one of three syntaxes: local syntax (`list warehouses where wh_id = 'MHE'`),
SQL (`[select * from wh_mst]`), and Groovy (`[[println 'hello']]`).

## Module

```
github.com/castingcode/mocka
```

Go version: see `go.mod`.

## Package Structure

```
mocka/            # All library code lives here as package mocka
  cmd/mockasrv/   # Binary entry point (thin wrapper only)
```

Everything except the binary is in the root `mocka` package. Consumers import
`github.com/castingcode/mocka` and get everything they need from a single package.
Do not create sub-packages to organize the library code — use files instead
(e.g., `session.go`, `response.go`, `matcher.go`, `query.go`).

## Architecture

See `docs/architecture.md` for the full design.

**Key rule:** Define interfaces in the consuming package (call-site interfaces),
not alongside the struct that satisfies them. Interfaces should be narrow —
only the methods the consuming package actually uses.

**Verify interface compliance:** When a concrete type is intended to implement an
interface, assert that at compile time as described in [Verify Interface Compliance](https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance)
in the Uber Go Style Guide. Use the zero value of the asserted type on the right-hand
side — for example `var _ http.Handler = (*Handler)(nil)` for pointer receivers, or
`var _ http.Handler = LogHandler{}` for value receivers. For interfaces defined only in
a consuming package, keep the assertion in that package (alongside the interface), not in
`mocka`.

## Error Handling

- Always wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- The colon-space before `%w` is required
- Do not use bare `errors.New` when you can provide context
- Do not swallow errors silently

## Logging

- Use `log/slog` for all structured logging
- Do not use `log.Default()`, `fmt.Print*`, or `log.Printf`
- Pass loggers via dependency injection; do not use a global logger
- Log at `Debug` for query matching details, `Info` for lifecycle events,
  `Warn` for unexpected-but-handled conditions, `Error` only for genuine failures

## Testing

- Use **GoConvey** (`github.com/smartystreets/goconvey`) for all tests
- Use `Convey` / `So` style — do not mix with `testing.T` assertions
- Table-driven tests are fine but should still use `Convey` blocks per case
- Test files live alongside the code they test (`matcher_test.go`, `session_test.go`)
- Integration tests should use the `//go:build integration` tag

## Query Normalization

All MOCA queries are normalized before matching:
- Lowercased
- Whitespace collapsed to single spaces
- Leading/trailing whitespace trimmed

Normalization happens once, at ingestion time for registered responses and at
lookup time for incoming queries. The `normalizeQuery` function in the root package owns this logic.

## Response File Format

Responses are defined in YAML files (see `docs/architecture.md` for schema).
Result sets are **always** referenced as paths to XML files — never embedded
inline in YAML. Queries may be inline (short) or referenced as file paths
(long queries). All paths are relative to the responses directory.

## What NOT To Do

- Do not add production-hardening concerns (auth, TLS, rate limiting, persistence).
  This is a test-support tool, not a production service.
- Do not special-case Groovy (`[[`) or SQL (`[`) syntax in the matching logic.
  All queries go through the same matching hierarchy.
- Do not define interfaces in the same package as the struct that satisfies them.
- Do not embed XML result sets inline in YAML response files.
- Do not use `log.Default()` or `fmt.Print*` for logging.
- Do not create sub-packages within the library to organize code — use files.
