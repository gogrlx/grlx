# CRUSH Configuration for grlx

## Build Commands
- `make all` - Build all binaries (sprout, grlx, farmer)
- `make sprout` - Build sprout binary
- `make grlx` - Build grlx CLI binary  
- `make farmer` - Build farmer binary
- `make clean` - Clean build artifacts

## Test Commands
- `go test ./...` - Run all tests
- `go test ./path/to/package` - Run tests for specific package
- `go test -run TestName ./path/to/package` - Run single test
- `go test -v ./...` - Run tests with verbose output

## Lint/Format Commands
- `go fmt ./...` - Format all Go code
- `golangci-lint run` - Run linter (if available)
- `go vet ./...` - Run Go vet

## Code Style Guidelines
- Use `any` instead of `interface{}` for modern Go
- Package imports: stdlib, third-party, local (with blank lines between groups)
- Error handling: return errors, don't panic in library code
- Naming: camelCase for unexported, PascalCase for exported
- Use context.Context for cancellation and timeouts
- Struct tags: json/yaml tags for serialization
- Test files: `*_test.go` with table-driven tests preferred