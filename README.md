# bkn-drive

A shared drive UI over [bkn](https://bkn.intrane.fr). Single Go binary, embedded
React, no build step and no runtime dependencies.

It is a client: the drive itself lives in bkn as two scripts and a hook
(`examples/drive/` in the bkn repo), which own the entries, quotas, groups and
sharing. This binary serves the interface and holds the session.

## Run

```sh
BKN_URL=https://bkn.intrane.fr bkn-drive serve --port 8080
```

`BKN_URL` defaults to `https://bkn.intrane.fr`. The server binds loopback
unless told otherwise, per cli-daemon-spec.

## Why the token lives here and not in the browser

Sign-in exchanges an email and password with bkn and keeps the resulting tokens
**in this process**, keyed by an opaque `HttpOnly` session cookie. The browser
never sees a bearer token. A drive is exactly the kind of thing worth that
arrangement: a token in `localStorage` is one XSS away from somebody else's
files, and the drive's whole promise is that it is not.

Sessions are in memory, so a restart signs everyone out. That is deliberate --
persisting them would mean writing 30-day refresh tokens to disk.

Access tokens last 15 minutes; the server refreshes them transparently, so a
session survives an afternoon of use.

## Endpoints

| route | does |
|---|---|
| `POST /api/login` | exchange credentials for a session cookie |
| `POST /api/logout` | drop the session |
| `GET /api/me` | who is signed in, and the upload limit |
| `POST /api/drive` | one drive op (`ls`, `mkdir`, `rm`, `mv`, `quota`, `share`, …) |
| `POST /api/upload` | multipart file -> the `drive-upload` hook |
| `GET /api/download` | 302 to a short-lived signed URL |

Downloads redirect rather than proxy, so bytes go straight from bkn to the
person and never through this process. Uploads cannot: bkn's hook takes base64
in JSON, so the file passes through memory here, which is why 25MB is the cap.

Administrative ops (`policy-set`, `policy-get`) are deliberately not exposed --
they need bkn's admin token, which this server does not hold.

## Storage

Blobs live in bkn's `drive-blobs` file namespace. bkn's local backend follows
`BKN_FILES_DIR`, and it also has an S3 backend, so moving the bytes elsewhere
is a bkn-side decision that needs no change here.
