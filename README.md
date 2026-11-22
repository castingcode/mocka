# Mocka — A Mock MOCA Server

[![Tests](https://github.com/castingcode/mocka/actions/workflows/test.yml/badge.svg)](https://github.com/castingcode/mocka/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/castingcode/mocka/graph/badge.svg?token=C7EUFCEHED)](https://codecov.io/gh/castingcode/mocka)
[![Go Report Card](https://goreportcard.com/badge/github.com/castingcode/mocka)](https://goreportcard.com/report/github.com/castingcode/mocka)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/castingcode/mocka/badge)](https://scorecard.dev/viewer/?uri=github.com/castingcode/mocka)

Mocka is an HTTP server that accepts MOCA protocol requests and returns canned responses. It is designed for testing code that talks to a BlueYonder WMS MOCA server without needing a real server present.

## Why Mocka?

Mocka is not a tool for testing your MOCA server. It is a tool for testing the code *you* write that calls a MOCA server.

Testing against a real MOCA server is slow and fragile — data changes, environments go down, and setting up known test state is expensive. Mocka gives every test request the same predictable response every time, so you can validate deserialization logic, error handling, null handling, date formatting, and similar concerns without any of that overhead.

What Mocka does **not** do is emulate side effects. If your command is supposed to move inventory, write audit records, trigger label printing, or fire any other downstream process, Mocka will not simulate any of that. It returns a result set and nothing else. If validating side effects is a requirement, you need a real MOCA server or a different tool.

In short: Mocka is the right tool when you want fast, deterministic, low-maintenance integration tests for your MOCA client code. It is not a substitute for end-to-end testing.

---

## Two ways to use Mocka

| Mode | When to use |
|---|---|
| **Go library** | Your project is written in Go and you want to embed the mock server directly in your test suite alongside `net/http/httptest`. |
| **Standalone binary** | Your project is written in another language, or you prefer to run the mock server as a separate process and configure it with YAML files. |

---

## Using Mocka as a Go library

### Installation

```sh
go get github.com/castingcode/mocka
```

### Quick start

Embed a mock server in a Go test using `net/http/httptest`:

```go
func TestMyMocaClient(t *testing.T) {
    loader := mocka.NewInMemoryResponseLoader(
        mocka.WithExactMatch(
            "list warehouses where wh_id = 'MHE'",
            mocka.NewResponse(mocka.StatusOK).
                WithResultSet(`<moca-results>
                    <metadata>
                        <column name="wh_id" type="S" length="10" nullable="false"/>
                        <column name="wh_name" type="S" length="40" nullable="true"/>
                    </metadata>
                    <data>
                        <row><field>MHE</field><field>Main Warehouse</field></row>
                    </data>
                </moca-results>`).
                Build(),
        ),
        mocka.WithExactMatch(
            "list warehouses where wh_id = 'UNKNOWN'",
            mocka.NewResponse(mocka.StatusSrvNoDataFound).
                WithMessage("No Data Found").
                Build(),
        ),
    )

    lookup, err := mocka.NewResponseLookup(loader)
    if err != nil {
        t.Fatal(err)
    }

    mux := http.NewServeMux()
    mocka.RegisterRoutes(mux, mocka.NewMocaRequestHandler(lookup))

    server := httptest.NewServer(mux)
    defer server.Close()

    // Point your MOCA client at server.URL and run your tests.
}
```

### Registering responses

All response registration goes through `NewInMemoryResponseLoader`. Pass any combination of the option functions below; they are applied in order and all append to the entry list.

#### `WithExactMatch`

Matches when the incoming query equals the registered query exactly (after normalization).

```go
mocka.WithExactMatch(
    "list warehouses where wh_id = 'MHE'",
    mocka.NewResponse(mocka.StatusOK).WithResultSet(xml).Build(),
)
```

#### `WithPrefixMatch`

Matches any query that begins with the registered prefix. Useful for commands where you want to return the same response regardless of the `where` clause.

```go
mocka.WithPrefixMatch(
    "list warehouses",
    mocka.NewResponse(mocka.StatusSrvNoDataFound).WithMessage("No Data Found").Build(),
)
```

#### `WithPublishDataMatch`

Matches a `publish data where ... | { inner command }` query by its inner command, regardless of what context keys the query carries. This is the generic fallback form.

```go
mocka.WithPublishDataMatch(
    "do thing",
    mocka.NewResponse(mocka.StatusOK).WithResultSet(xml).Build(),
)
```

#### `WithContextualPublishDataMatch`

Matches a `publish data where ... | { inner command }` query by both its inner command and a specific set of context key/value pairs. Takes priority over `WithPublishDataMatch` for the same inner command when the context keys match.

```go
mocka.WithContextualPublishDataMatch(
    "do thing",
    map[string]string{"wh_id": "MHE"},
    mocka.NewResponse(mocka.StatusOK).WithResultSet(mheXML).Build(),
)
```

#### `WithEntries`

Low-level escape hatch for passing a pre-built `[]mocka.Entry` slice — useful when sharing fixtures across tests.

```go
mocka.WithEntries(sharedFixtures)
```

### Building responses

Use `NewResponse` to construct a `Response` value for any of the option functions above.

```go
// Status only (for commands that return no data on success)
mocka.NewResponse(mocka.StatusOK).Build()

// Status and message (for error responses)
mocka.NewResponse(mocka.StatusSrvNoDataFound).WithMessage("No Data Found").Build()

// Status and inline result XML
mocka.NewResponse(mocka.StatusOK).WithResultSet(xmlString).Build()

// Status and result XML loaded from a file
rb, err := mocka.NewResponse(mocka.StatusOK).WithResultSetFromFile("testdata/warehouses.xml")
if err != nil {
    t.Fatal(err)
}
resp := rb.Build()
```

Result XML files should contain a `<moca-results>` fragment. See [Result file format](#result-file-format) for the schema.

### Status code constants

| Constant | Value | Meaning |
|---|---|---|
| `StatusOK` | 0 | Success |
| `StatusSrvNoDataFound` | 510 | Server-level no data found |
| `StatusDBNoDataFound` | -1403 | Database-level no data found |
| `StatusDBError` | 511 | Database error |
| `StatusGroovyException` | 531 | Groovy script exception |
| `StatusCommandNotFound` | 501 | No registered response matched |
| `StatusInvalidSessionKey` | 523 | Missing or invalid session key |

---

## Using Mocka as a standalone binary

### Installation

Download a pre-built binary from the [releases page](https://github.com/castingcode/mocka/releases), build from source:

```sh
go install github.com/castingcode/mocka/cmd/mockasrv@latest
```

or run via Docker:

```sh
docker run -p 4500:4500 -v ./responses:/responses castingcode/mocka
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-port` | `4500` | Port to listen on |
| `-folder` | `./responses` next to the binary | Directory containing `responses.yml` |
| `-validate` | `false` | Load and validate the response files, then exit (status 1 on error) without serving. Useful in CI. |

### Directory layout

```
responses/
  responses.yml          # response definitions (responses.yaml also works; not both)
  responses.d/           # optional — more response files, loaded in file name order
    10-warehouses.yml
    20-orders.yml
  warehouses-mhe.xml     # result XML files (paths are relative to this directory)
  queries/
    long-query.txt       # optional — query text files for long queries
```

Every `*.yml` and `*.yaml` file in `responses.d/` uses the same schema as `responses.yml`, so you can group responses by feature. `results` and `query_file` paths are always relative to the responses directory, even in files under `responses.d/`.

### Validation

Response files are validated when the server starts, and any problem stops it with an error naming the file and entry:

- unknown `match.type` values and misspelled field names
- a missing `query`/`query_file`, `inner`, or `prefix` for the entry's type, or both `query` and `query_file`
- fields that belong to a different match type (for example `query` on a `prefix` entry)
- missing `query_file` or `results` files
- duplicate entries — the same exact query, prefix, or publish_data inner and context — within or across files

Run `mockasrv -validate -folder ./responses` to check the files without starting the server. If no response files are found, the server still starts and logs a warning.

### responses.yml schema

```yaml
responses:
  - match:
      type: exact
      query: "list warehouses where wh_id = 'MHE'"   # inline query
    response:
      status: 0
      results: warehouses-mhe.xml                     # path to result XML, relative to responses dir

  - match:
      type: exact
      query_file: queries/long-query.txt              # query text in a separate file
    response:
      status: 0
      results: long-query-result.xml

  - match:
      type: publish_data
      inner: "do thing"                               # the command inside { }
      context:                                        # optional — omit for generic fallback
        wh_id: MHE
        prt_client_id: ACME
    response:
      status: 0
      results: do-thing-mhe-acme.xml

  - match:
      type: publish_data
      inner: "do thing"                               # no context = matches any context
    response:
      status: 0
      results: do-thing-generic.xml

  - match:
      type: prefix
      prefix: "list warehouses"
    response:
      status: 510
      message: No Data Found                          # message only, no result file

  - match:
      type: exact
      query: "explode"
    response:
      status: 511
      message: "Database Error"
```

### Result file format

Result files contain a `<moca-results>` XML fragment. No XML declaration is needed.

```xml
<moca-results>
    <metadata>
        <column name="wh_id"   type="S" length="10" nullable="false"/>
        <column name="wh_name" type="S" length="40" nullable="true"/>
    </metadata>
    <data>
        <row>
            <field>MHE</field>
            <field>Main Warehouse</field>
        </row>
    </data>
</moca-results>
```

Common MOCA column types: `S` (string), `I` (integer), `F` (float), `D` (date), `R` (flag/boolean).

---

## Match types and query normalization

All queries — incoming requests and registered entries alike — are normalized before comparison:

- **Lowercased** — matching is always case-insensitive
- **Whitespace collapsed** — newlines, tabs, and multiple spaces are treated as a single space
- **Bracket whitespace trimmed** — leading/trailing whitespace inside `[...]` (SQL) and `[[...]]` (Groovy) blocks is normalized
- **Quotes canonicalized in local syntax** — outside of SQL/Groovy brackets, single quotes and double quotes are interchangeable: `where a = 'foo'` and `where a = "foo"` match the same entry

These rules apply identically for all three MOCA syntaxes (local, SQL, Groovy). You do not need to worry about exact whitespace or quote style when registering responses.

### Match hierarchy

For each incoming query, Mocka evaluates registered entries in this order, returning the first match:

1. **Exact** — the normalized query equals a registered `type: exact` entry
2. **Publish-data contextual** — the query is a `publish data where ... | { ... }` form, and a `type: publish_data` entry matches both the inner command and all of the entry's context key/value pairs; when several match, the one with the most context pairs wins (the first listed breaks a tie)
3. **Publish-data generic** — same form, but a `type: publish_data` entry with no context matches the inner command alone
4. **Prefix** — the normalized query starts with a registered `type: prefix` string; when several prefixes match, the longest wins regardless of file order
5. **No match** — returns status `501` (command not found)

---

## Built-in commands

These commands are handled by Mocka directly and do not need to be registered:

| Command | Behavior |
|---|---|
| `ping` | Returns status 0 with no result set |
| `login user where usr_id = '...' and usr_pswd = '...'` | Creates a session and returns a standard login result set including a `session_key` |
| `logout user` | Destroys the session identified by `SESSION_KEY` in the request environment |

All other commands require a `SESSION_KEY` environment variable in the MOCA request. Requests without a valid session key receive status `523`.

---

## Admin API

The standalone binary serves these endpoints next to `/service`. Go library users get them with `mocka.RegisterAdminRoutes(mux, handler)`. They need no session key.

### `POST /_mocka/match` — test a query

Shows how a query would be handled without sending a MOCA request. The body is the query text, or `{"query": "..."}` with `Content-Type: application/json`.

```sh
curl -s localhost:4500/_mocka/match -d "LIST warehouses where wh_id = 'MHE'"
```

```json
{
  "query": "LIST warehouses where wh_id = 'MHE'",
  "normalized_query": "list warehouses where wh_id = 'mhe'",
  "match": {"type": "exact", "query": "list warehouses where wh_id = 'mhe'", "source": "responses.yml entry 1"},
  "response": {"status": 0, "results": "<moca-results>...</moca-results>"}
}
```

`match` is `null` when nothing matches; `response` then shows the `501` the client would receive. Built-in commands report `{"type": "builtin", "command": "login"}` and no `response`. For a registered entry, `match.id` is the id the endpoints below use.

### `GET /_mocka/requests` — see what was received

Returns the MOCA requests received so far, oldest first, as `{"requests": [...]}`. Each record has `time`, `query` (as sent), `normalized_query`, `status`, `match` (same shape as above, or `null`), and `unmatched`. Mocka keeps the most recent 1000.

Add `?unmatched=true` to list only the queries no registered response matched — the quickest way to find out what to add to your response files:

```sh
curl -s "localhost:4500/_mocka/requests?unmatched=true"
```

### `DELETE /_mocka/requests` — clear the log

Returns `204`. Useful between tests.

### `GET /_mocka/responses` — list registered responses

Returns `{"responses": [...]}` in match order. Each item has the same `match` (with `id` and `source`) and `response` shape as `/_mocka/match`.

### `POST /_mocka/responses` — register a response at runtime

Lets a test add or override a response without editing files. `match` takes the same fields as `responses.yml` (except `query_file`); `response.results` is the `<moca-results>` XML itself, not a file path:

```sh
curl -s localhost:4500/_mocka/responses -H 'Content-Type: application/json' -d '{
  "match": {"type": "exact", "query": "list carriers where carcod = '"'UPS'"'"},
  "response": {"status": 0, "results": "<moca-results><metadata>...</metadata><data>...</data></moca-results>"}
}'
```

The entry is validated the same way as the response files, and the result XML must parse. If an entry already matches the same queries (the same exact query, prefix, or publish_data inner and context), it is replaced in place and the request returns `200`; otherwise the entry is added and the request returns `201`. Both return the stored entry, whose `match.id` you can delete later. Runtime entries have `"source": "admin API"` and last until they are deleted or the responses are reloaded.

### `DELETE /_mocka/responses/{id}` — remove a response

Returns `204`, or `404` when no entry has that id. File entries can be removed too, until the next reload.

### `POST /_mocka/reload` — reload the response files

Re-reads `responses.yml` and `responses.d/`, discarding runtime entries, and returns `{"count": N}`. Edit the files and reload instead of restarting the server. If the files are invalid, the request returns `500` with the error and the current responses are kept. Ids are never reused, so an id from before a reload never refers to a different entry.

In Go, `handler.Requests()` and `handler.ClearRequests()` give the same access without HTTP, for example to assert which queries your code sent:

```go
for _, rec := range handler.Requests() {
    if rec.Unmatched {
        t.Errorf("no response registered for %q", rec.Query)
    }
}
```

---

## Contributing

Contributions are welcome. Please open an issue or submit a pull request on [GitHub](https://github.com/castingcode/mocka).

## License

This project is licensed under the MIT License.
