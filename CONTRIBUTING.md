# Contributing to Engram

Thanks for your interest in contributing to Engram! Here's how to get started.

## Development Setup

```bash
# Clone the repo
git clone https://github.com/erikmeyer/engram.git
cd engram

# Build
go build -o engram ./cmd/engram

# Run tests
go test ./...
```

**Requirements:** Go 1.23+, buf (for proto generation)

## Making Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Run the race detector: `go test -race ./...`
6. Commit with a descriptive message
7. Open a pull request against `main`

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Error handling: return errors, don't panic. Wrap with `fmt.Errorf("context: %w", err)`
- Every public function accepts `context.Context` as its first parameter
- Table-driven tests with `testify` for assertions
- Structured logging with `slog` — no `fmt.Println` in production code

## Proto Changes

If you modify `.proto` files in `proto/engram/v1/`:

```bash
buf generate
buf lint
buf breaking --against .git#branch=main
```

Never reuse field numbers. Reserve removed fields.

## Reporting Issues

Use [GitHub Issues](https://github.com/erikmeyer/engram/issues). Include:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Engram version (`engram --version`)
- OS and Go version

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
