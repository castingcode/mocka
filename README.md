# Mocka — A Mock MOCA Server

[![Tests](https://github.com/castingcode/mocka/actions/workflows/test.yml/badge.svg)](https://github.com/castingcode/mocka/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/castingcode/mocka/graph/badge.svg?token=C7EUFCEHED)](https://codecov.io/gh/castingcode/mocka)
[![Go Report Card](https://goreportcard.com/badge/github.com/castingcode/mocka)](https://goreportcard.com/report/github.com/castingcode/mocka)

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

    gin.SetMode(gin.TestMode)
    router := gin.New()
    mocka.RegisterRoutes(router, mocka.NewMocaRequestHandler(lookup))

    server := httptest.NewServer(router)
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

Download a pre-built binary from the [releases page](https://github.com/castingcode/mocka/releases), or build from source:

```sh
go install github.com/castingcode/mocka/cmd/mockasrv@latest
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-port` | `9000` | Port to listen on |
| `-folder` | `./responses` next to the binary | Directory containing `responses.yml` |

### Directory layout

```
responses/
  responses.yml          # required — global response definitions
  warehouses-mhe.xml     # result XML files (paths are relative to this directory)
  queries/
    long-query.txt       # optional — query text files for long queries
```

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
2. **Publish-data contextual** — the query is a `publish data where ... | { ... }` form, and a `type: publish_data` entry matches both the inner command and all of the entry's context key/value pairs
3. **Publish-data generic** — same form, but a `type: publish_data` entry with no context matches the inner command alone
4. **Prefix** — the normalized query starts with a registered `type: prefix` string
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

## Contributing

Contributions are welcome. Please open an issue or submit a pull request on [GitHub](https://github.com/castingcode/mocka).

## License

This project is licensed under the MIT License.
