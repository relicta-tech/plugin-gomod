# GoMod Plugin for Relicta

Official GoMod plugin for [Relicta](https://github.com/relicta-tech/relicta) - Publish Go modules to proxy.golang.org.

## Installation

```bash
relicta plugin install gomod
relicta plugin enable gomod
```

## Configuration

Add to your `release.config.yaml`:

```yaml
plugins:
  - name: gomod
    enabled: true
    config:
      # Add configuration options here
```

## License

MIT License - see [LICENSE](LICENSE) for details.
