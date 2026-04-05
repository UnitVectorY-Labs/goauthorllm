# Install

## Requirements

- Go 1.26.1 or newer
- A terminal that supports Bubble Tea applications

## From A Local Checkout

Build the binary from the repository root:

```bash
go build .
```

Run the resulting binary directly:

```bash
./goauthorllm --file draft.md
```

If you omit `--file`, the app starts in a chooser view for markdown files in the current working directory.

## Install Into Your Go Bin

If you want a local install from the checked-out source tree:

```bash
go install .
```

Run that command from the repository root. Make sure `$GOBIN` or `$GOPATH/bin` is on your `PATH` if you want to launch the binary globally.

## Runtime Configuration

The app reads `.env` automatically when present, then consults environment variables and flags for runtime settings. See [Usage](USAGE.md) for the full list.
