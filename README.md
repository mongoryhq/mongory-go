# mongory-go

Go bindings and extensions for Mongory Core (C implementation).

## Installation

```bash
go get github.com/mongoryhq/mongory-go
```

## Usage

```go
import "github.com/mongoryhq/mongory-go"
```

> Note: This project is in the initialization phase; the API is subject to change.

## Dependencies and System Requirements

- Go 1.24
- If/when CGO integration with `mongory-core` is enabled, a C toolchain may be required (e.g., clang/llvm, make).

## Versioning Policy

- Use v0.x while the API is unstable
- Release v1 once stable
- For v2+, use semantic import paths (e.g., `github.com/mongoryhq/mongory-go/v2`)

## License

License: MIT

