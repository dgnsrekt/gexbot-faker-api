# Demos

Scriptable terminal demos of the CLIs, rendered to GIFs with
[charmbracelet VHS](https://github.com/charmbracelet/vhs). The `.tape` scripts are
the source of truth — deterministic and re-renderable when the tools change.

## Layout

- `cli/*.tape` — VHS scripts (declarative: `Type`, `Sleep`, `Set Theme`, …).
- Rendered GIFs are written to `website/public/demos/*.gif` (committed), where the
  guides site serves them at `{base}/demos/*.gif` and the README references them.

## Regenerate

```bash
just demos-render
```

This renders every `cli/*.tape` against a **running faker on `:8080`** (start one
first with `just serve-gex-faker` or `docker compose up -d`). `gexfakercli` must be
on your `PATH` (`go install ./cmd/gexfakercli` or `just build-gexfakercli` + add
`bin/` to PATH).

## One-time toolchain

```bash
go install github.com/charmbracelet/vhs@latest        # vhs -> ~/go/bin
sudo dnf install ttyd jetbrains-mono-fonts             # Fedora (ttyd is the terminal VHS drives)
# ffmpeg is also required (usually already installed)
```

VHS drives a real terminal (`ttyd`) and encodes with `ffmpeg`; the theme in each
tape matches the GEX Faker Studio brand (near-black + green, JetBrains Mono).

## Tapes

- `cli/gexfakercli.tape` — the agent golden flow: `setup` → `status` → two
  `classic` pulls (the playback cursor advances) → `reset`.

More tapes (e.g. `docker compose up`, the downloader, `describe`) drop in as new
`cli/*.tape` files and are picked up by `just demos-render`.
