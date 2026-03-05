export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    console.log(`[fetch] ${request.method} ${url.pathname}`);

    if (url.pathname === "/health") {
      return new Response("ok", { status: 200 });
    }

    if (request.headers.get("Upgrade") !== "websocket") {
      console.warn(`[fetch] Expected WebSocket upgrade, got: ${request.headers.get("Upgrade")}`);
      return new Response("Expected WebSocket upgrade", { status: 400 });
    }

    // Extract vault from path: /vault-id or default
    const vault = url.pathname.slice(1) || "default";
    console.log(`[fetch] WebSocket connection for vault: ${vault}`);
    const id = env.SYNC_RELAY_ROOM.idFromName(vault);
    const stub = env.SYNC_RELAY_ROOM.get(id);
    return stub.fetch(request);
  },
};

const WATCH_ADD_PREFIX = "+";
const WATCH_REMOVE_PREFIX = "-";

export class SyncRelayRoom {
  constructor(state, env) {
    this.state = state;
    this.env = env;
    this.connectionCounter = 0;
  }

  async fetch(request) {
    const connId = ++this.connectionCounter;
    console.log(`[SyncRelayRoom.fetch#${connId}] ✓ NEW CONNECTION OPENED`);
    if (request.headers.get("Upgrade") !== "websocket") {
      console.warn(`[SyncRelayRoom.fetch#${connId}] Expected WebSocket upgrade`);
      return new Response("Expected WebSocket upgrade", { status: 400 });
    }

    const pair = new WebSocketPair();
    const client = pair[0];
    const server = pair[1];

    const currentWebSockets = this.state.getWebSockets();
    console.log(`[SyncRelayRoom.fetch#${connId}] Current connected clients: ${currentWebSockets.length}, Accepting new WebSocket connection`);
    
    server.serializeAttachment(JSON.stringify({ folders: [], connId }));
    this.state.acceptWebSocket(server);
    
    const afterWebSockets = this.state.getWebSockets();
    console.log(`[SyncRelayRoom.fetch#${connId}] After accept, total clients: ${afterWebSockets.length}`);

    return new Response(null, { status: 101, webSocket: client });
  }

  webSocketMessage(ws, message) {
    const raw = ws.deserializeAttachment();
    const data = raw ? JSON.parse(raw) : {};
    const connId = data.connId || "unknown";
    
    if (typeof message !== "string") {
      console.warn(`[webSocketMessage#${connId}] Received non-string message`);
      return;
    }

    if (message.startsWith(WATCH_ADD_PREFIX)) {
      const add = message.slice(WATCH_ADD_PREFIX.length);
      if (!add) {
        console.warn(`[webSocketMessage#${connId}] Empty folder in add request`);
        return;
      }
      const current = getFolders(ws);
      const next = mergeFolders(current, [add]);
      console.log(`[webSocketMessage#${connId}] Added folder: ${add}. Current folders: ${next.join(", ")}`);
      ws.serializeAttachment(JSON.stringify({ folders: next, connId }));
      return;
    }

    if (message.startsWith(WATCH_REMOVE_PREFIX)) {
      const remove = message.slice(WATCH_REMOVE_PREFIX.length);
      if (!remove) {
        console.warn(`[webSocketMessage#${connId}] Empty folder in remove request`);
        return;
      }
      const current = getFolders(ws);
      const next = current.filter((f) => f !== remove);
      console.log(`[webSocketMessage#${connId}] Removed folder: ${remove}. Remaining folders: ${next.join(", ")}`);
      ws.serializeAttachment(JSON.stringify({ folders: next, connId }));
      return;
    }

    const message_raw = message;
    if (!message_raw) {
      console.warn(`[webSocketMessage#${connId}] Empty message`);
      return;
    }

    // Parse message format: "vaultID:clientID:filename"
    const colonIdx1 = message_raw.indexOf(":");
    if (colonIdx1 < 0) {
      console.warn(`[webSocketMessage#${connId}] Invalid message format (missing first colon): "${message_raw}"`);
      return;
    }
    const colonIdx2 = message_raw.indexOf(":", colonIdx1 + 1);
    if (colonIdx2 < 0) {
      console.warn(`[webSocketMessage#${connId}] Invalid message format (missing second colon): "${message_raw}"`);
      return;
    }
    const vaultID = message_raw.substring(0, colonIdx1);
    const clientID = message_raw.substring(colonIdx1 + 1, colonIdx2);
    const filename = message_raw.substring(colonIdx2 + 1);
    
    if (!vaultID || !clientID || !filename) {
      console.warn(`[webSocketMessage#${connId}] Invalid message format: vaultID="${vaultID}", clientID="${clientID}", filename="${filename}"`);
      return;
    }

    console.log(`[webSocketMessage#${connId}] Broadcasting change: vaultID="${vaultID}", clientID="${clientID}", filename="${filename}"`);
    const sockets = this.state.getWebSockets();
    let sent = 0;
    for (const socket of sockets) {
      const socketData = socket.deserializeAttachment();
      const socketConnId = socketData ? JSON.parse(socketData).connId : "unknown";

      const folders = getFolders(socket);
      console.log(`[webSocketMessage#${connId}] Checking socket connId#${socketConnId} with folders: ${JSON.stringify(folders)}`);
      
      const matches = matchesAnyFolder(filename, folders);
      if (matches) {
        console.log(`[webSocketMessage#${connId}] ✓ Match! Sending to connId#${socketConnId} watching: ${folders.join(", ")}`);
        socket.send(message_raw);
        sent++;
      } else {
        console.log(`[webSocketMessage#${connId}] ✗ No match. connId#${socketConnId}: "${filename}" does not match folders: ${JSON.stringify(folders)}`);
      }
    }
    console.log(`[webSocketMessage#${connId}] Sent notification to ${sent} socket(s) out of ${sockets.length} total`);
  }

  webSocketClose(ws) {
    const data = ws.deserializeAttachment();
    const connId = data ? JSON.parse(data).connId : "unknown";
    const remainingSockets = this.state.getWebSockets();
    console.log(`[webSocketClose#${connId}] WebSocket closed. Remaining clients: ${remainingSockets.length}`);
    try {
      ws.close();
    } catch {
      // ignore
    }
  }

  webSocketError(ws) {
    console.error(`[webSocketError] WebSocket error`);
    try {
      ws.close();
    } catch {
      // ignore
    }
  }
}

function getFolders(ws) {
  const raw = ws.deserializeAttachment();
  if (!raw) {
    return [];
  }
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed.folders) ? parsed.folders : [];
  } catch {
    return [];
  }
}

function mergeFolders(current, incoming) {
  const set = new Set(current);
  for (const folder of incoming) {
    set.add(folder);
  }
  return Array.from(set.values());
}

function matchesAnyFolder(name, folders) {
  console.log(`[matchesAnyFolder] Checking if "${name}" matches any of: ${JSON.stringify(folders)}`);
  for (const folder of folders) {
    const exactMatch = name === folder;
    const prefixMatch = name.startsWith(folder + "/");
    console.log(`[matchesAnyFolder] - Folder "${folder}": exact="${exactMatch}", startsWith="${prefixMatch}"`);
    if (exactMatch || prefixMatch) {
      console.log(`[matchesAnyFolder] → MATCH found with "${folder}"`);
      return true;
    }
  }
  console.log(`[matchesAnyFolder] → NO MATCH found`);
  return false;
}
