# Architecture

## Purpose

Mocka is a Go library that provides a mock MOCA server for use in integration tests.
Consuming projects import it to start an in-process (or out-of-process via the binary)
HTTP server that accepts MOCA protocol requests and returns canned responses defined
in YAML + XML files on disk.

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
| `http_handler.go` | HTTP routing, built-in command handling (ping, login, logout) |
| `matcher.go` | Query matching hierarchy |
| `query.go` | Query normalization (`normalizeQuery`) |
| `session.go` | In-memory session store |
| `response.go` | Response types, YAML loading, XML file resolution |

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
4. Delegates all other queries to `response.Registry`
5. Marshals the response back to MOCA XML

Login and logout are handled in the handler, not in the response registry.
They are intentionally not configurable via response files.

## Query Matching Hierarchy

All incoming queries are normalized (lowercased, whitespace collapsed) before matching.
Registered response queries are normalized at load time. Matching is therefore
case-insensitive and whitespace-insensitive throughout. See `normalizeQuery` in `query.go`.

The `response.Matcher` evaluates candidates in this order, returning the first match:

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

Responses are defined in a single YAML file (by convention `responses/responses.yml`).
User-specific overrides live in `responses/user_responses.yml` under a per-user key.
User-specific responses are checked before global ones at each stage of the hierarchy.

### Schema

```yaml
responses:
  - match:
      type: exact
      query: "list warehouses where wh_id = 'MHE'"   # inline query (short)
    response:
      status: 0
      results: list-warehouses-mhe.xml                # path to XML file

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

### User-Specific Overrides

```yaml
# user_responses.yml
CYCLEUSER07:
  responses:
    - match:
        type: exact
        query: "list warehouses"
      response:
        status: 0
        results: cycleuser07-warehouses.xml
```

User responses are checked first at each matching stage. If no user-specific match
is found, the global registry is used.

## Session Management

Sessions are stored in memory in a `map[string]string` (session key → user ID).
There is no TTL, no eviction, and no concurrency protection. This is intentional:
mocka is a single-use test server, not a production service.

Session keys are UUIDs generated at login time.

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/castingcode/mocaprotocol` | MOCA XML envelope types |
| `github.com/gin-gonic/gin` | HTTP routing |
| `github.com/google/uuid` | Session key generation |
| `gopkg.in/yaml.v3` | Response file parsing |
| `github.com/smartystreets/goconvey` | Tests |
| `log/slog` (stdlib) | Structured logging |

## Design Constraints

- **No production hardening.** No TLS, no auth, no rate limiting, no persistence.
- **Library first.** The exported API surface should be usable for embedding; the
  binary is a thin convenience wrapper.
- **Flat package structure.** All library code is in the root `mocka` package.
  Use files to separate concerns, not sub-packages.
- **No special query type handling.** SQL and Groovy queries go through the same
  matching hierarchy as local syntax. If they aren't registered, they return 501.
- **Normalization is the source of truth.** Any comparison between two query strings
  must go through `normalizeQuery`. Never compare raw strings.
