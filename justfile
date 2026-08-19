# standardgo carries the ruleset and the golangci-lint engine in one binary, so this
# repo holds no lint config of its own. It is run via `go run`, not a go.mod tool
# directive, to keep it out of this library's dependency graph.
standardgo := "github.com/amberpixels/standardgo/cmd/standardgo@v0.1.2"

# The floor this library promises to support. Keep in sync with the `go` directive
# in go.mod - that is the number consumers actually see. CI passes FLOOR_GO=local,
# having already installed the floor toolchain, so it never downloads a second one.
floor_go := env("FLOOR_GO", "go1.25.0")

# Default recipe: format. Rewrites files, but decides nothing - every change it
# makes is mechanical. `just fix` is the one that applies judgement.
default: fmt

# format Go code - rewrites to canonical form
fmt:
    go run {{ standardgo }} fmt ./...

# lint Go code - reports findings, changes nothing
lint:
    go run {{ standardgo }} ./...

# auto-fix what can be fixed - run on a clean tree and read the diff
fix:
    go run {{ standardgo }} ./... --fix

# run tests
test:
    go test ./...

# check that the go.mod floor still builds and vets. vet compiles the tests too,
# so this covers the test dependencies at the floor as well.
floor:
    GOWORK=off GOTOOLCHAIN={{ floor_go }} go build ./...
    GOWORK=off GOTOOLCHAIN={{ floor_go }} go vet ./...

# build
build:
    go build ./...

# run all checks - read-only, safe for CI
ci: lint test floor
