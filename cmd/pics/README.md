
Whenever you modify files in the data directory, run `go generate`
to regenerate `.go` files with embedded resources.  You will need 
`github.com/jteeuwen/go-bindata` installed.

To get a fully statically compiles pics command binary, use:

```bash
go build -ldflags="-extldflags=-static" -tags sqlite_omit_load_extension
```
