# Sync Relay Server (Cloudflare Workers + Durable Objects)

WebSocket-based sync relay server for real-time file change notifications across vault instances.

## Connect

WebSocket endpoint:

- `wss://<your-domain>/<vault-id>`

Each `vault-id` creates an isolated Durable Object instance. Clients connecting to the same vault-id can notify each other of file changes.

## Client messages (text)

- Add watched folder: send a single text message starting with `+`, followed by
  the folder name.

Example:

```
+blockchain
+data
```

- Remove watched folder: send a single text message starting with `-`, followed by
  the folder name.

```
-blockchain
```

- Notify change: send message in format `vaultID:clientID:filename`

```
users@test:0x1234567890:blockchain/file.txt
```

## Server messages (text)

- Change event: server sends the message in format `vaultID:clientID:filename`

```
users@test:0x1234567890:blockchain/file.txt
```

## Matching

A client receives a change if:
1. The vaultID matches their subscribed vault
2. The clientID is different from their own (no echo)
3. The filename matches a watched folder (exact match or starts with `<folder>/`)

## Notes

- Each vault gets an isolated Durable Object via path-based routing
- Single WebSocket connection can handle multiple vaults efficiently
- Non-text messages are ignored
- WebSocket attachments store per-connection state (hibernation-friendly)
- Client-side filtering by vaultID and clientID prevents echo and cross-vault leaks
