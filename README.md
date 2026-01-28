# gopkgcp

A utility tool to extract Go packages and their dependencies into a standalone module.

## Installation

```bash
# Install goda (required dependency)
go install github.com/loov/goda@latest

# Build gopkgcp
go build -o gopkgcp ./cmd
```

## Usage

```bash
gopkgcp -pkg <package> -o <output-dir> [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-pkg` | (required) | Package path to extract (e.g., `./responses`) |
| `-o` | (required) | Output directory |
| `-mod` | | Override module name in extracted files |
| `-module-only` | `true` | Only extract packages from the same module (exclude external deps) |
| `-v` | `false` | Verbose output |

## Examples

### Basic extraction

```bash
gopkgcp -pkg ./responses -o ./extracted
```

### Extract with custom module name

```bash
gopkgcp -pkg ./responses -o ./extracted -mod github.com/myorg/myproject
```

This replaces all imports from the original module (e.g., `github.com/openai/openai-go/v3`) with `github.com/myorg/myproject` in all `.go` files and `go.mod`.

### Include external dependencies

```bash
gopkgcp -pkg ./responses -o ./extracted -module-only=false
```

### Verbose output

```bash
gopkgcp -pkg ./responses -o ./extracted -v
```

## What it does

1. Uses `goda` to analyze package dependencies
2. Copies all required packages to the output directory
3. Copies `go.mod` and `go.sum`
4. If `-mod` is specified, replaces the module name in all files
5. Runs `go mod tidy` to clean up dependencies

## Output

The extracted package is a self-contained Go module that can be used independently.
