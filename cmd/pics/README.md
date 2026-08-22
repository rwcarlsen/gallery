Assets under `data/` are embedded with Go's `embed` package.

To get a fully statically compiled pics binary:

```bash
go build -ldflags="-extldflags=-static" -tags sqlite_omit_load_extension
```
