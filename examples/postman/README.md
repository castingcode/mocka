# Mocka Postman Example

This directory contains two [Postman](https://www.postman.com/) collections for a running `mockasrv` instance.

[`mocka.postman_collection.json`](mocka.postman_collection.json) (**Mocka MOCA Server**) walks through the core MOCA request flow:

1. **Ping** — built-in health check (no session)
2. **Login** — creates a session and captures the `session_key`
3. **List Warehouses** — a registered command that requires a valid session
4. **List Locations (CSV)** — a registered command whose result set comes from a CSV file
5. **Unregistered Command** — a command with no entry in `responses.yml`, which returns status 501
6. **Logout** — ends the session

[`mocka-admin.postman_collection.json`](mocka-admin.postman_collection.json) (**Mocka Admin API**) uses the JSON endpoints under `/_mocka/`:

1. **Match Query** — shows which entry a query matches and the response it gets, without a session
2. **Match Query (JSON, no match)** — the same endpoint with a JSON body, for a query nothing matches
3. **List Requests** — the requests received so far and what handled each one
4. **List Unmatched Requests** — only the queries with no registered response (request 5 of the MOCA collection)
5. **List Responses** — the registered responses with their ids
6. **Register Response** — adds a response at runtime for the command MOCA request 5 sent
7. **Match Registered Query** — that command now matches the new response
8. **Delete Response** — removes it by id
9. **Reload Responses** — re-reads `responses.yml` and drops runtime entries
10. **Clear Request Log** — empties the log

Run the MOCA collection first: requests 3 and 4 of the admin collection check the log it fills.

Sample response data lives in [`responses/`](responses/).

## Prerequisites

- [Postman](https://www.postman.com/downloads/) (desktop app or CLI)
- Either [Go](https://go.dev/) (to run locally) or [Docker](https://www.docker.com/) (to run in a container)

## Sample data

```
responses/
  responses.yml              # maps queries to result files
  list-warehouses-mhe.xml    # canned result for the list-warehouses request
  list-locations-mhe.csv     # canned result for the list-locations request, as CSV
```

The collection sends `list warehouses where wh_id = 'MHE'` and `list locations where wh_id = 'MHE'`, which are registered as exact matches in `responses.yml`; the second uses a [CSV result file](../../README.md#csv-result-files). Edit these files to try different queries and result sets — see the [response file format](../../docs/architecture.md#response-file-format) in the architecture docs.

## Run mockasrv locally

From the repository root:

```sh
go run ./cmd/mockasrv -folder examples/postman/responses
```

By default the server listens on port **4500**. Override with `-port` if needed:

```sh
go run ./cmd/mockasrv -folder examples/postman/responses -port 8080
```

If you built or installed the binary instead:

```sh
mockasrv -folder examples/postman/responses
```

You should see a log line similar to:

```
level=INFO msg="mocka server listening" addr=[::]:4500 responses=.../examples/postman/responses
```

## Run mockasrv with Docker

Run from the repository root so the volume mount path resolves correctly. Mount the sample responses directory at `/responses` — the default data path inside the container.

### Pre-built image (Docker Hub)

The fastest option. Pull and run [`castingcode/mocka`](https://hub.docker.com/r/castingcode/mocka) from Docker Hub:

```sh
docker run --rm -p 4500:4500 \
  -v "$(pwd)/examples/postman/responses:/responses" \
  castingcode/mocka:latest
```

### Build locally

If you are working from a checkout and want to run the image you just built:

```sh
docker build -t mocka .
docker run --rm -p 4500:4500 \
  -v "$(pwd)/examples/postman/responses:/responses" \
  mocka
```

### Custom host port

To expose the server on a different host port (either image name works):

```sh
docker run --rm -p 8080:4500 \
  -v "$(pwd)/examples/postman/responses:/responses" \
  castingcode/mocka:latest
```

Then set the collection variable `baseUrl` to `http://localhost:8080`.

## Import and run the collections

1. Open Postman.
2. **Import** → select [`mocka.postman_collection.json`](mocka.postman_collection.json) and [`mocka-admin.postman_collection.json`](mocka-admin.postman_collection.json).
3. Confirm the collection variable `baseUrl` is `http://localhost:4500` (or your chosen port) in both collections.
4. With `mockasrv` running, execute the **Mocka MOCA Server** requests **in order** (1 → 6), then the **Mocka Admin API** requests (1 → 10).

The **Login** request saves the returned `session_key` to the `sessionKey` collection variable. Requests 3 to 6 use that value automatically. The admin requests need no session. If you skip login or restart the server, run **Login** again before continuing.

You can also run the collection from the CLI:

```sh
npx --yes newman run examples/postman/mocka.postman_collection.json
npx --yes newman run examples/postman/mocka-admin.postman_collection.json
```

CI runs both collections the same way against a freshly built `mockasrv` (the `postman` job in [`.github/workflows/test.yml`](../../.github/workflows/test.yml)), so keep them passing when you change the collections, the sample responses, or the server.

## Admin API

The admin endpoints take and return JSON (`/_mocka/match` also accepts the query as a plain-text body). See [Admin API](../../README.md#admin-api) in the main README for the full description.

## Request format

All requests are `POST` to `/service` with `Content-Type: application/moca-xml`. The body is a MOCA XML envelope:

```xml
<moca-request autocommit="true">
  <environment>
    <var name="SESSION_KEY" value="your-session-key-here"></var>
  </environment>
  <query><![CDATA[list warehouses where wh_id = 'MHE']]></query>
</moca-request>
```

Commands other than `ping` and `login user` require a valid `SESSION_KEY` in the environment block. Login and logout are built into the server and do not need entries in `responses.yml`.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Connection refused | `mockasrv` is not running, or `baseUrl` port does not match `-port` / Docker mapping |
| HTML error page instead of XML | Missing or wrong `Content-Type` header (must be `application/moca-xml`) |
| MOCA status 523 | Missing or expired session — run **Login** again |
| MOCA status 501 | Query not registered in `responses.yml` — send it to `/_mocka/match` to see how it was normalized, or list `/_mocka/requests?unmatched=true` |
