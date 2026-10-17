# ext-parser-go

Go parser plugin for **Onfly**. Built on Go's own `go/parser` and `go/ast` (no tree-sitter, no CGO). Compiles to a WASI module for Onfly's wazero sandbox, or a native binary for stdio use.

```bash
onfly ext install go
```

Handles `.go`.

## Chunks

Input is one file on stdin, argv[1] is its path, output is the standard `[]Chunk` JSON.

| `type` | Source node | Notes |
|---|---|---|
| `function` | `func` declaration | doc comment included |
| `method` | `func` with a receiver | `receiver` = the receiver type, pointer or value |
| `struct` `interface` `type` | `type` declaration | doc comment included; `type` for aliases and other kinds |

Metadata: `name`, and `receiver` for methods. Chunks carry no line numbers, namespace or signature.

## Known limitations

- A file that does not parse fails: the plugin exits 1 and writes the error to stderr, so that file keeps its previous chunks. Other plugins chunk what parsed.
- No size limits, no splitting of long bodies, no `file` fallback: a file with no functions or types yields `[]`.
- `const`, `var` and import blocks are not chunked.

## Build

```bash
make wasm     # bin/go-parser.wasm (Go 1.27+)
make native   # bin/go-parser
go test ./...
```

## Use

```yaml
indexer:
  include: ["**/*.go"]
  parsers:
    - extensions: [".go"]
      type: "wasm"
      path: ".onfly/plugins/go-parser.wasm"
```

To run the native binary as a subprocess instead, build it with `make native` and use `type: "stdio"` with `command: "./.onfly/plugins/go-parser"`.

## Example

Input (`main.go`):

```go
package main

type Service struct{}

func (s *Service) Execute() error {
    return nil
}
```

Output:

```json
[
  {
    "file_path": "main.go",
    "type": "struct",
    "content": "type Service struct{}",
    "metadata": {"name": "Service"}
  },
  {
    "file_path": "main.go",
    "type": "method",
    "content": "func (s *Service) Execute() error {\n    return nil\n}",
    "metadata": {"name": "Execute", "receiver": "Service"}
  }
]
```

The chunk contract is described in the [`cli` docs](https://github.com/OnFlyApp/cli/blob/main/docs/parsers.md). Plugins are listed in the [registry](https://github.com/OnFlyApp/registry).

## Author

[Oleksii Ivanov](https://github.com/axitdev)

## License

Licensed under the **Functional Source License, Version 1.1, ALv2 Future License** (SPDX: `FSL-1.1-ALv2`), see [LICENSE](LICENSE).

Free for any purpose other than a competing use: making the software available in a commercial product or service that substitutes for it. Each version converts to Apache 2.0 two years after it is released.
