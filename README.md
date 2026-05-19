# Declaw Go SDK

Go client SDK for [Declaw](https://declaw.ai) — security-first sandboxing for AI agents.

## Installation

```bash
go get github.com/declaw-ai/declaw-go
```

Requires Go 1.22+.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    declaw "github.com/declaw-ai/declaw-go"
)

func main() {
    ctx := context.Background()

    sbx, err := declaw.Create(ctx, declaw.WithTimeout(300))
    if err != nil {
        log.Fatal(err)
    }
    defer sbx.Kill(ctx)

    result, err := sbx.Commands.Run(ctx, "echo hello from declaw")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result.Stdout)
}
```

Set your API key via environment variable:

```bash
export DECLAW_API_KEY=your-api-key
```

## Documentation

Full reference: [docs.declaw.ai/sdks/go](https://docs.declaw.ai/sdks/go/overview)

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.
