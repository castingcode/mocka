# Architecture

## Purpose

Mocka is a Go library that provides a mock MOCA server for use in integration tests.
Consuming projects import it to start an in-process HTTP server (via `net/http/httptest`)
or run the standalone binary (`mockasrv`) out-of-process. Either way, it accepts MOCA
protocol requests and returns canned responses.

The library is **not** a production service. Simplicity and ease of test setup are
preferred over robustness, concurrency safety, and security hardening.

## Package Layout

```
mocka/            # All library code — package mocka
  cmd/mockasrv/   # Binary wrapper — package main
```

All library code lives in the root `mocka` package. Internal concerns are separated
by file rather than by sub-package:

| File | Responsibility |
|---|---|
| `http_handler.go` | HTTP routing, `Router` interface, built-in command handling (ping, login, logout) |
| `registry.go` | `ResponseLookup` — resolves normalized queries to responses |
| `matcher.go` | Query matching hierarchy (`matchQuery`) |
| `query.go` | Query normalization (`normalizeQuery`) |
| `session.go` | In-memory session store |
| `response.go` | Core types: `Response`, `Entry`, `MatchType`, status constants |
| `response_loader.go` | `ResponseLoader` interface |
| `response_builder.go` | Fluent `ResponseBuilder` for programmatic response construction |
| `response_loader_inmemory_adapter.go` | `InMemoryResponseLoader` and its option functions |
| `response_loader_yaml_file_adapter.go` | `FileResponseLoader` — loads responses from YAML + XML files on disk |

This keeps the consumer import surface simple: `import "github.com/castingcode/mocka"`.

### `normalizeQuery`

Lowercases, collapses whitespace to single spaces, and trims. Applied to both
incoming queries and registered queries at load time. Any string comparison between
two MOCA queries must go through `normalizeQuery` — never compare raw strings.

## HTTP Handler

The handler (`MocaRequestHandler`) lives at the root library level and is the primary
exported type consumers use. It:

1. Validates `Content-Type: application/moca-xml`
2. Parses the MOCA XML envelope via `mocaprotocol`
3. Handles `ping`, `login user`, and `logout user` as built-in commands
4. Delegates all other queries to `ResponseLookup`
5. Marshals the response back to MOCA XML

Login and logout are handled in the handler, not in the response registry.
They are intentionally not configurable via response files.

### Router Interface

`RegisterRoutes` accepts a `Router` interface rather than a concrete framework type:

```go
type Router interface {
    HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
    Handle(pattern string, handler http.Handler)
}
```

`*http.ServeMux` satisfies this interface out of the box. This keeps the library
free of HTTP framework dependencies while remaining compatible with any router that
follows the standard `net/http` handler signature.

## Response Loaders

The `ResponseLoader` interface is the primary extension point for supplying canned responses:

```go
type ResponseLoader interface {
    Load() (entries []Entry, err error)
}
```

Two implementations are provided:

### `InMemoryResponseLoader`

The preferred approach for Go test suites. Responses are registered programmatically
using option functions, with no files on disk required:

```go
loader := mocka.NewInMemoryResponseLoader(
    mocka.WithExactMatch("list warehouses where wh_id = 'MHE'", resp),
    mocka.WithPrefixMatch("list warehouses", fallbackResp),
    mocka.WithPublishDataMatch("do thing", resp),
    mocka.WithContextualPublishDataMatch("do thing", map[string]string{"wh_id": "MHE"}, resp),
    mocka.WithEntries(sharedFixtures),
)
```

All option functions normalize queries at registration time, so callers do not need
to worry about case or whitespace.

### `FileResponseLoader`

Used by the standalone `mockasrv` binary. Reads a `responses.yml` file from a
directory on disk, with result XML loaded from paths relative to that directory.
A missing `responses.yml` is treated as an empty registry; a malformed one returns
an error.

## Query Matching Hierarchy

All incoming queries are normalized (lowercased, whitespace collapsed) before matching.
Registered response queries are normalized at load time. Matching is therefore
case-insensitive and whitespace-insensitive throughout. See `normalizeQuery` in `query.go`.

`matchQuery` in `matcher.go` evaluates candidates in this order, returning the first match:

### 1. Exact Match

The full normalized query string matches a registered entry exactly.

```
list warehouses where wh_id = 'mhe'
```

### 2. Publish-Data Contextual Match

Applies when the query matches this structure:

```
publish data where <key> = <value> [and <key> = <value> ...] | { <inner query> }
```

The matcher:
1. Extracts the key/value pairs from the `publish data where` clause (order-insensitive)
2. Extracts the inner query from inside `{ }` and normalizes it
3. Looks for a registered entry with `type: publish_data`, matching `inner` query
   AND all `context` key/value pairs
4. Falls back to a registered entry with `type: publish_data`, matching `inner` query
   only (no context), if the contextual match fails

Nesting (multiple levels of `publish data`) is not supported at this time.

### 3. Prefix Match

The normalized query starts with a registered prefix string.

```yaml
- match:
    type: prefix
    prefix: "list warehouses"
```

This matches `list warehouses where wh_id = 'abc'` and any other query beginning
with that string after normalization.

### 4. No Match

Returns `StatusCommandNotFound` (501). No special handling for SQL or Groovy syntax —
they fall through the same hierarchy.

## Response File Format

Used by `FileResponseLoader` and the standalone binary. Responses are defined in a
single `responses.yml` file; result XML is stored in separate files referenced by path.

### Schema

```yaml
responses:
  - match:
      type: exact
      query: "list warehouses where wh_id = 'MHE'"   # inline query (short)
    response:
      status: 0
      results: list-warehouses-mhe.xml                # path to XML file, relative to responses dir

  - match:
      type: exact
      query_file: queries/long-query.txt              # file reference (long queries)
    response:
      status: 0
      results: long-query-result.xml

  - match:
      type: publish_data
      inner: "do thing"                              # normalized inner command
      context:                                       # optional; omit for generic fallback
        wh_id: "MHE"
        prt_client_id: "ACME"
    response:
      status: 0
      results: do-thing-mhe-acme.xml

  - match:
      type: publish_data
      inner: "do thing"                             # no context = generic fallback
    response:
      status: 0
      results: do-thing-generic.xml

  - match:
      type: prefix
      prefix: "list warehouses"
    response:
      status: 510
      message: No Data Found                        # message inline; results omitted

  - match:
      type: exact
      query: "explode"
    response:
      status: 511
      message: "Database Error"
```

### Result Files

- `results` is a path relative to the responses directory
- The file contains a `<moca-results>` XML fragment (no XML declaration needed)
- Omit `results` entirely for responses that return only a status and message

## Session Management

Sessions are stored in memory in a `map[string]string` (session key → user ID).
There is no TTL, no eviction, and no concurrency protection. This is intentional:
mocka is a single-use test server, not a production service.

Session keys are UUIDs generated at login time.

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/castingcode/mocaprotocol` | MOCA XML envelope types |
| `github.com/goccy/go-yaml` | Response file parsing |
| `github.com/google/uuid` | Session key generation |
| `github.com/smartystreets/goconvey` | Tests |
| `log/slog` (stdlib) | Structured logging |
| `net/http` (stdlib) | HTTP routing via `Router` interface and `*http.ServeMux` |

## Design Constraints

- **No production hardening.** No TLS, no auth, no rate limiting, no persistence.
- **Library first.** The exported API surface should be usable for embedding; the
  binary is a thin convenience wrapper.
- **Flat package structure.** All library code is in the root `mocka` package.
  Use files to separate concerns, not sub-packages.
- **No HTTP framework dependency.** The `Router` interface accepts any router
  compatible with the standard `net/http` handler signature, keeping framework
  choice with the consumer.
- **No special query type handling.** SQL and Groovy queries go through the same
  matching hierarchy as local syntax. If they aren't registered, they return 501.
- **Normalization is the source of truth.** Any comparison between two query strings
  must go through `normalizeQuery`. Never compare raw strings.
