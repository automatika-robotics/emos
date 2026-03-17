# EMOS CLI

The command-line interface for [EMOS](https://github.com/automatika-robotics/emos) — the Embodied Operating System.

> **Full documentation at [emos.automatikarobotics.com](https://emos.automatikarobotics.com/getting-started/cli.html)**

## Install

```bash
curl -sSL https://raw.githubusercontent.com/automatika-robotics/emos/main/stack/emos-cli/scripts/install.sh | sudo bash
```

## Build from Source

Requires Go 1.23+.

```bash
cd stack/emos-cli
make build && sudo make install
```

## Development

```bash
make build       # Build for current platform
make build-all   # Cross-compile linux/amd64 + arm64
make tidy        # go mod tidy
make clean       # Remove build artifacts
```

## License

MIT. See [LICENSE](../../LICENSE) for details.
