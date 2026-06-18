# Mocka Postman Example

This directory contains a [Postman](https://www.postman.com/) collection that walks through the core MOCA request flow against a running `mockasrv` instance:

1. **Ping** — built-in health check (no session)
2. **Login** — creates a session and captures the `session_key`
3. **List Warehouses** — a registered command that requires a valid session
4. **Logout** — ends the session

Sample response data lives in [`responses/`](responses/).

## Prerequisites

- [Postman](https://www.postman.com/downloads/) (desktop app or CLI)
- Either [Go](https://go.dev/) (to run locally) or [Docker](https://www.docker.com/) (to run in a container)

## Sample data

```
responses/
  responses.yml              # maps queries to result files
  list-warehouses-mhe.xml    # canned result for the list-warehouses request
```

The collection sends `list warehouses where wh_id = 'MHE'`, which is registered as an exact match in `responses.yml`. Edit these files to try different queries and result sets — see the [response file format](../../docs/architecture.md#response-file-format) in the architecture docs.

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

## Import and run the collection

1. Open Postman.
2. **Import** → select [`mocka.postman_collection.json`](mocka.postman_collection.json).
3. Confirm the collection variable `baseUrl` is `http://localhost:4500` (or your chosen port).
4. With `mockasrv` running, execute the requests **in order** (1 → 4).

The **Login** request saves the returned `session_key` to the `sessionKey` collection variable. Requests 3 and 4 use that value automatically. If you skip login or restart the server, run **Login** again before continuing.

You can also run the collection from the CLI:

```sh
npx --yes newman run examples/postman/mocka.postman_collection.json
```

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
| MOCA status 501 | Query not registered in `responses.yml` |
