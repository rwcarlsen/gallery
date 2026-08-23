# Agents

Read `README.md` first — spirit, usage, pipe protocol. This file is how to work in the tree.

Branch `go-mod` is the real main. `master` is frozen ~2018.

## Keep the docs current

`README.md` is for humans (build, use, maintain). This file is for agents working in the tree. Update **both** in the same change whenever you add, remove, or alter a command, flag, pipe rule, schema, layout, invariant, build step, or gotcha. Do not let them go stale. If a section in either file would now be wrong, fix it before you finish.

## Layout

| change | put it |
|---|---|
| SQL, search math, notes, users | `piclib/` |
| ncnn, preprocess, model fetch | `internal/infer/` |
| CLI verb | `cmd/pics/<verb>.go` + register in `cmds` |
| HTTP | `webpics.go` routes, `context.go` session, `data/` templates + `static/` |
| tests | `piclib/*_test.go` next to the code |

Do not add a second binary. Do not add a config format. Do not put originals in the DB. `libdata/` is empty — ignore it.

## Invariants

- One binary, many verbs. New capability is a `pics` subcommand unless it cannot be.
- Flags + env only (`PICLIB`, `HOME`).
- Schema is additive `CREATE TABLE/INDEX IF NOT EXISTS` or a one-shot `ALTER` you write yourself. No version table. `Open` is implicit migrate.
- Notes are a `meta` log, not an UPDATE. `SetMeta` is a no-op if unchanged.
- Originals chmod `0444` after ingest. Never rename except `pics fix -fnames`.
- Prefer composing `list` over adding flags to `serve`.
- Prefer a SQLite column over a sidecar file, except originals and ncnn weights.
- `check(err)` fatals. Subcommands do not return errors. `add` is the exception and continues.
- Templates stay tiny. JS is vanilla `fetch` in two files. No frontend framework.
- `log.SetFlags(0)`. Usage on stderr via `log.Printf`. Every verb gets `newFlagSet` and one sentence of description.

## Pipe protocol

`WriteLines` / `ParseLines` / `idsOrStdin` in `cmd/pics/util.go`.

- First whitespace field is an integer pic ID (or face ID for `person name`).
- Args, if present, win over stdin.
- `add` stdin is paths, split with `strings.Fields`.
- `index` with no args and no stdin = all pics.
- Right-click → `GET /dynamic/clickpic/{id}` → print ID on **server** stdout. Intentional.

## Extending

**New verb:** `func foo(cmd string, args []string)` + `cmds["foo"] = foo`. Own `flag.FlagSet` via `newFlagSet`. Take IDs from `idsOrStdin` unless the verb is about paths.

**New HTTP route:** Go 1.22+ method+path patterns (`GET /photo/{kind}/{id}`). Session state lives on `gallery` (`visible` = auth-filtered, `photos` = search-filtered, `why` = per-pic search reasons). Do not invent a second session store.

**New table:** add it to the `Open` `CREATE TABLE IF NOT EXISTS` block and to the schema comment in `piclib/doc.go`.

**No `pics rm`.** Checksums, thumbs, clip, faces, and notes all hang off `files.id`. Do not invent delete casually.

## Verify

```bash
go test ./piclib
```

Needs CGO + sqlite3, not ncnn models. Helpers: `testLib` → `t.TempDir()` + `Open`; fake JPEGs (`0xff 0xd8 0xff …`) so `AddFile` does not need real images (thumb errors are ignored).

No tests for `cmd/pics`, HTTP, or `internal/infer`. Do not add a test stack that needs models.

`CGO_ENABLED=0` will not build.

## Gotchas

1. **`.gitignore` line `pics`** matches any path named `pics`, so **new files under `cmd/pics/` are invisible to git**. Already-tracked files still update. Build with `go build -o ../../pics` from `cmd/pics` — never output to `cmd/pics/pics`. To add a new file there: `git add -f cmd/pics/new.go`, or fix the ignore to `/pics`.

2. **CLIP/face search runs after date+regex.** `pics list -similar x -from 2020` only ranks that year.

3. **`NeedsFaces` is “zero rows”, not “wrong model”.** Old-model faces will not re-index unless `-force`. `NeedsClip` *does* check the model name. CLIP embeds the in-DB thumb (original if empty); faces use the original. Switching CLIP to thumbs does not bump the model name — already-indexed rows stay until `pics index -clip -force`.

4. **`ReplaceFaces` deletes `face_person` for that pic+model.** Re-index wipes names on that photo; `AutoAssignPic` may restore them.

5. **Web search engine is process-global** (`searchEng`). Fine for one `serve`.

6. **Sessions die on restart.** Cookie HMAC key (`session.key`) does not.

7. **User visibility is notes-only.** Faces/people do not filter who sees a photo. Zero users ⇒ no login. First `user add` turns it on.

8. **`AddFile` swallows thumb errors.** Non-images can enter with empty thumbs.

9. **Filenames with spaces break `pics add` from stdin.**

10. **`third_party/ncnn` is sqlite-style vendored C++.** `go build` compiles it via cgo (first compile is slow). Re-vendor with `scripts/vendor-ncnn.py /path/to/ncnn-src` (keeps `custom.cpp`). Weights are converted with `scripts/convert-models.sh` (needs pnnx); `pics index -fetch DIR` copies the expected names into `$PICLIB/models/`. Custom layers `Tensor.to` / `torch.where` / `Gather` / `Embed` / `Gemm` live in `third_party/ncnn/custom.cpp` for the CLIP text graph. `Embed` is overwritten so token ids written as float32 values (`float32(id)`) are read as `(int)float` — stock ncnn Embed bitcasts the bits and clamps every token to EOT. `Gemm` is overwritten so a 3D `c=1` activation (pnnx `2=1` reshape after attn) is treated as 2D; stock Gemm takes `M=c=1` and broadcasts the SOT row, so every text query yields the same EOT vector. Restart `serve` after a rebuild.

11. **`view`/`serve` use `xdg-open`.** Linux-centric.

12. **HTTP uses Go 1.22+ patterns.** Keep that.

13. **Covering indexes on `files` omit `thumb`.** List queries must not pull the blob. Thumbs are JPEG in-DB; face chips quality 85.

14. **Magic numbers, do not “tune” without measuring:** CLIP 224 + OpenAI mean/std; SCRFD 640 letterbox, det 0.5, NMS 0.4, min face 16px; ArcFace 112, 5-point align; defaults 0.2 / 0.35; 24 thumbs/page; list names truncated to 44 with `...` prefix.

15. **Model names stored with rows:** `clip-vit-b32`, `buffalo-s`. Changing the name retriggers CLIP; faces need `-force`.

16. **`clickpic` prints to the server process, not the browser.**

## Style already in the tree

- `piclib.Lib` = one directory + `*sql.DB`. `Pic.id` unexported, `Pic.Id` exported for templates.
- Vectors: little-endian float32, L2-normalized. Cosine = dot product.
- Face boxes normalized `[0,1]`. Chips pad 15% when cropped.
- Auth: bcrypt, HMAC cookies `gallery-sess=id.hmac`, HttpOnly, SameSite=Lax.
- Dates stored as unix seconds. Missing EXIF → Unix 0; time-nav skips those.
