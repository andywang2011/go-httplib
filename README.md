# go-httplib

[![test](https://github.com/andywang2011/go-httplib/actions/workflows/test.yml/badge.svg)](https://github.com/andywang2011/go-httplib/actions/workflows/test.yml)

HTTP client with structured outbound request logging, built for use with [go-logging](https://github.com/andywang2011/go-logging).

## Install

```bash
go get github.com/andywang2011/go-httplib@v0.1.0
```

## Quick start

```go
client := httplib.NewClient(httplib.WithTimeout(5 * time.Second))
fmt.Println(client.Timeout > 0)
```

## Examples

```bash
go test -run Example ./...
cd go-httplib/examples/outbound && go run .
```

## License

MIT — see [LICENSE](LICENSE).
