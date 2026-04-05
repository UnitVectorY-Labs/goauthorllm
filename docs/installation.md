---
layout: default
title: Installation
nav_order: 2
---

# Installation

## Requirements

- Go 1.26 or newer
- A terminal that supports alternate screen mode (most modern terminals)

## Build From Source

Clone the repository and build the binary:

```bash
git clone https://github.com/UnitVectorY-Labs/goauthorllm.git
cd goauthorllm
go build .
```

Run the resulting binary:

```bash
./goauthorllm
```

## Install to Go Bin

Install from the repository root so the binary is placed in your Go bin directory:

```bash
go install .
```

Make sure `$GOBIN` or `$GOPATH/bin` is on your `PATH` to run `goauthorllm` from anywhere.

## GitHub Releases

Pre-built binaries are available on the [GitHub Releases](https://github.com/UnitVectorY-Labs/goauthorllm/releases) page for Linux, macOS, and Windows across multiple architectures.

## Verify the Installation

Check the installed version:

```bash
goauthorllm --version
```
