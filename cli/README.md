# Bao CLI

Native text client for Bao vault and replica operations.

## Run

```bash
cd /Users/ea/Documents/Lab/section31/bao/cli
GOCACHE=/tmp/go-build go run .
```

Or one-shot commands:

```bash
GOCACHE=/tmp/go-build go run . id-new
GOCACHE=/tmp/go-build go run . open --private <PRIVATE_ID> --creator <CREATOR_PUBLIC_ID> --config /path/to/store.yaml
GOCACHE=/tmp/go-build go run . tui
```

## Structure

- `main.go`: entrypoint
- `types.go`: app/session types + shared helpers
- `commands.go`: command handlers (vault/replica/identity)
- `shell.go`: interactive command shell parser
- `tui.go`: full-screen TUI mode

## TUI Keys

- `q`: quit TUI
- `tab`, `1`, `2`: switch pane (Vault / Replica)
- `o`: open vault from saved session
- `p`: open replica from saved session
- `r`: refresh current pane
- `s`: sync (vault or replica, based on pane)
- `j`/`k` or arrows: move selection
- `enter`:
  - Vault pane: open directory / download file
  - Replica pane: preview selected table
- `u`: go up one directory (vault pane)
- `?`: toggle help overlay

Session defaults are persisted in `~/.bao/cli-session.yaml`.
