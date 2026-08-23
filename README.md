# gallery

A personal photo archive. Originals stay on disk as ordinary files. SQLite is only an index. The web UI is a viewer, not a manager.

One binary, `pics`, does everything: ingest, search, notes, people, users, and the HTTP gallery. IDs on stdin/stdout are the API — compose small commands instead of growing flags on `serve`.

## Spirit

- Content-addressed store: ingest hashes the file, names it `{sha256}{ext}`, chmod `0444`.
- The database catalogs; it does not vault. Do not put originals in SQLite.
- Small tools composed by pipes. Prefer `list | view` over new flags on `serve`.
- Intelligence is optional. CLIP and faces are a fetchable sidecar, not a service.
- Auth is optional. Zero users ⇒ open gallery. First `user add` turns on login.
- No config file. `PICLIB` + flags. No second binary. No cloud. No delete command.

2014, still true: *all file-based management — web interface is only for viewing.*

## Requirements

- Go 1.25+
- A C compiler. CGO is required (`mattn/go-sqlite3`). `CGO_ENABLED=0` will not build.
- ONNX Runtime (`libonnxruntime.so`) only if you run `pics index`. The rest of the CLI and the web UI work without it.
- Linux is the intended host. `view` / `serve -view` call `xdg-open`.

## Build

From the repo root:

```bash
cd cmd/pics && go build -o ../../pics
```

Write the binary to the repo root, not `cmd/pics/pics`. A `.gitignore` line `pics` matches any path of that name, so a binary left in `cmd/pics/` is easy to lose track of.

Point `pics` at a library directory (checked in this order: `-lib`, `$PICLIB`, `~/.piclib`):

```bash
export PICLIB=~/Photos
pics -h
```

Static binary (still needs CGO + sqlite at compile time):

```bash
cd cmd/pics && go build -o ../../pics -ldflags="-extldflags=-static" -tags sqlite_omit_load_extension
```

See `cmd/pics/README.md` for the embed note. Templates and static files under `cmd/pics/data/` are compiled in.

### Models (optional)

CLIP search and face naming need ONNX weights. They live in `$PICLIB/models/`, not the repo:

```bash
pics index -fetch               # CLIP ViT-B/32 + InsightFace buffalo_s
```

`libonnxruntime.so` lookup: `ONNXRUNTIME_LIB`, then `$PICLIB/models/`, then `/usr/lib`.

## First use

```bash
export PICLIB=~/Photos
mkdir -p "$PICLIB"
pics add ~/incoming/*.jpg
pics serve -all -view           # browse at 127.0.0.1:7777
```

Later, if you want text search and named people:

```bash
pics index -fetch && pics index
pics list -similar 'red kayak' | pics view
```

## Library

```
$PICLIB/
  <sha256>.jpg                  # originals, write-once
  piclib.sqlite                 # catalog (WAL, page_size 8192)
  session.key                   # 32 bytes, HMAC for cookies
  models/                       # ONNX + BPE, optional
```

Schema is in `piclib/doc.go`. Tables are created `IF NOT EXISTS` on every `Open`. There is no version table and no migration runner — schema changes are additive `CREATE` or a one-shot `ALTER` you write yourself. Notes are an append-only `meta` log; the latest row for a field wins.

There is no `pics rm`. Checksums, thumbs, CLIP vectors, faces, and notes all hang off `files.id`.

## The pipe

`list` and `person show` print:

```
<id>  <unix>  <YYYY/M/D>  "<truncated-name>"  <json-escaped notes>
```

Most verbs take pic IDs from args, or from stdin if no args are given. The first whitespace field is the ID.

Exceptions:

- `add` stdin is **paths** (`strings.Fields` — spaces in names break).
- `index` with no args and no stdin means **every pic**.
- Right-click a thumb in the web UI → the **server** process prints that ID on its stdout.

```bash
pics list -from 2019 -to 2019 | pics view
pics list -person ada | pics copy -dst ~/export/ada
pics list -similar 'red kayak' | pics serve -view
```

## Commands

`pics [opts] <cmd> [opts] [args]`. Global: `-lib`. Live flags: `pics <cmd> -h`.

| cmd | does |
|---|---|
| `add` | copy files in; skip dups (`[ADD]` / `[SKIP]` / `[ERROR]`) |
| `list` | filter by date, notes/name regex, CLIP, person, face |
| `validate` | re-hash; `[ERROR]` on mismatch. `-all`, `-v` |
| `fix` | `-untracked` print orphans; `-fnames` rename hash-matches |
| `link` | symlink `{Name}-{Id}`; `-tree` → `{dst}/{year}/{Month}/` |
| `copy` | real copy `{stem}-{Id}{ext}` |
| `note` | print, append (`-text`), or `-replace` (empty clears) |
| `meta` | catalog + EXIF tags from the original (`id  key  value`) |
| `serve` | HTTP on `127.0.0.1:7777`; `-all`, `-view`, `-noedit` |
| `view` | like serve, ephemeral port, always opens browser |
| `index` | CLIP + faces; `-clip` / `-faces` / `-force` / `-fetch` |
| `person` | `name` / `list` / `show` / `rm` |
| `user` | `add` / `list` / `set` / `passwd` / `rm` |

`link` and `copy` refuse to overwrite. `serve`/`view` skip `.avi`/`.m4v`. Both block forever.

### `list`

```bash
pics list -from 2018 -to 2019 -note 'vacation|italy'
pics list -similar 'snowy cabin' -min 0.25
pics list -like 1842
pics list -person ada
pics list -face 77 -min 0.4
```

Dates go through `dateparser`. Empty `-from` → year 0; empty `-to` → now. A partial `-from` defaults to Jan 1 this year; a partial `-to` defaults to Dec 31 23:59:59.

CLIP/face **reorder** the date+regex set. They do not search the whole library if you also passed `-from`. Defaults: CLIP min `0.2`, face min `0.35`. Needs `pics index` first.

### `index` / `person`

```bash
pics index -fetch && pics index
pics list -from 2024 | pics index -faces
pics person name ada 101 102          # assign + propagate lookalikes
pics person show ada | pics view
pics person rm ada                    # unname; face rows stay
```

Person identity is a centroid + threshold, not a training loop. Re-index (`-force`) wipes names on that photo; auto-assign may restore them.

### `user`

```bash
pics user add alice -write -notes 'vacation|kids' -hide-empty
pics user list
pics user set alice -write false
pics user passwd alice
```

Password is prompted on a TTY; `-pass` is for scripts. Visibility is a Go regex on the latest notes. Faces do not affect who sees a photo. `CanWrite` gates notes and face naming in the UI.

## Web

`serve`/`view` load a pic-id set (`-all`, args, or stdin). Each browser session then filters that set (auth → search → person/face). 24 thumbs per page. Notes and face chips are editable unless `-noedit`.

The search box is notes + named people + CLIP (CLIP optional). Zero users ⇒ no login. Sessions are in-memory (restart logs everyone out); `session.key` only verifies the cookie.

## Maintain

- **Integrity:** `pics validate -all` re-hashes originals and prints `[ERROR]` on mismatch. Use `-v` to print `[VALID]` too.
- **Orphans / bad names:** `pics fix -untracked` lists files in `$PICLIB` that are not in the catalog (including `piclib.sqlite`, `session.key`, `models/`). `pics fix -fnames` renames hash-matching orphans back to `{sha256}{ext}` — the only supported rename.
- **Schema:** additive only. New tables go in the `Open` `CREATE TABLE IF NOT EXISTS` block and in the comment in `piclib/doc.go`.
- **Tests:** `go test ./piclib`. Needs CGO + sqlite3, not ONNX. There are no tests for `cmd/pics`, HTTP, or `internal/infer`.
- **No delete.** Do not add `pics rm`. Everything hangs off `files.id`.

## Architecture

```
$PICLIB  ── piclib.Lib ── pics CLI ── stdin/stdout IDs
                │              │
                │         infer.Engine (ONNX, lazy)
                │              │
                └──────── HTTP gallery (embed.FS, stdlib)
```

| pkg | role |
|---|---|
| `cmd/pics` | flag dispatch, pipe protocol, HTTP, embed |
| `piclib` | library, SQL, filter, CLIP/face math, users |
| `internal/infer` | CLIP + SCRFD + ArcFace |
| `third_party/onnxruntime_go` | local `replace` of the ORT Go wrapper |

CLIP/face search is brute-force cosine in Go. Fine for a personal library. Vectors are little-endian float32, L2-normalized.

## Limits

- No photo-delete API. Checksums, thumbs, clip, faces, notes all hang off `files.id`.
- `view` uses `xdg-open` (Linux).
- `add` stdin cannot quote paths.
- Covering indexes omit `thumb`, so listing stays cheap. Don't `SELECT *` the files table in new code.
