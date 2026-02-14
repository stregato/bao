package vault

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/stregato/bao/lib/core"
	"golang.org/x/net/websocket"
)

const watchQueueSize = 1024

func (v *Vault) startSyncRelay() error {
	if v.syncRelayCh != nil || v.Config.SyncRelay == "" {
		return nil // Sync relay already running or not configured
	}

	// Open or reuse the websocket connection to the sync relay server
	client, err := getOrCreateWatchClient(v.Config.SyncRelay)
	if err != nil {
		return err
	}

	client.mu.Lock()
	if _, exists := client.subscribers[v.ID]; !exists {
		if err := websocket.Message.Send(client.conn, watchAddPrefix+v.Realm.String()); err != nil {
			client.mu.Unlock()
			return core.Error(core.NetError, "cannot send watch add message", err)
		}
		client.subscribers[v.ID] = make(map[string]*Vault)
	}
	instanceID := fmt.Sprintf("%p", v)
	client.subscribers[v.ID][instanceID] = v
	client.mu.Unlock()
	v.syncRelayCh = make(chan string, watchQueueSize)

	go v.relayLoop()
	return nil
}

func (v *Vault) relayLoop() {
	defer v.cleanupSyncRelay()

	v.Sync()
	blockchainPrefix := path.Join(v.Realm.String(), BlockChainFolder)
	dataPrefix := path.Join(v.Realm.String(), DataFolder)

	for name := range v.syncRelayCh {
		switch {
		case strings.HasPrefix(name, blockchainPrefix+"/"):
			v.syncBlockChain()
		case strings.HasPrefix(name, dataPrefix+"/"):
			now := core.Now()
			dir, name := path.Split(name)
			dir = path.Clean(dir)
			_, err := v.syncronizeFile(dir, name)
			if err != nil {
				core.Error("failed to sync file %s/%s/%s: %v", v.ID, dir, name, err)
			} else {
				v.lastSyncAt = now
			}
		}
	}
}

func (v *Vault) cleanupSyncRelay() {
	client, err := getOrCreateWatchClient(v.Config.SyncRelay)
	if err != nil {
		return
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	instanceID := fmt.Sprintf("%p", v)
	subs := client.subscribers[v.ID]
	if subs == nil {
		return
	}

	// Remove this vault instance from the subscriber list
	delete(subs, instanceID)

	// If no more subscribers for this vault, clean up
	if len(subs) == 0 {
		delete(client.subscribers, v.ID)
		websocket.Message.Send(client.conn, watchRemovePrefix+v.Realm.String())

		// If no more subscribers for any vault, close the websocket
		if len(client.subscribers) == 0 {
			dropWatchClient(v.Config.SyncRelay, client)
		}
	}
}

func (v *Vault) stopSyncRelay() {
	if v.syncRelayCh == nil {
		return // Sync relay not running, nothing to do
	}

	close(v.syncRelayCh)
	v.syncRelayCh = nil
}

const watchAddPrefix = "+"
const watchRemovePrefix = "-"

type syncClient struct {
	conn *websocket.Conn
	// Subscribers grouped by vault ID.
	subscribers map[string]map[string]*Vault
	mu          sync.Mutex
}

var watchClientsMu sync.Mutex
var watchClients = map[string]*syncClient{}

func (v *Vault) notifyChange(filename string) error {
	core.Start("filename %s", filename)
	if v.syncRelayCh == nil {
		core.End("sync relay not running, skipping notify")
		return nil // Sync relay not running, nothing to do
	}
	name := strings.TrimSpace(filename)
	if name == "" {
		return core.Error(core.GenericError, "filename is empty")
	}

	client, err := getOrCreateWatchClient(v.Config.SyncRelay)
	if err != nil {
		return err
	}
	instanceID := fmt.Sprintf("%p", v)
	// Include vaultID and instanceID with notification: format is vaultID:instanceID:filename
	message := v.ID + ":" + instanceID + ":" + name
	if err := websocket.Message.Send(client.conn, message); err != nil {
		dropWatchClient(v.Config.SyncRelay, client)
		return core.Error(core.NetError, "cannot send notify message", err)
	}
	core.End("")
	return nil
}

func readWatchEvents(conn *websocket.Conn, server string) {
	defer conn.Close()

	for {
		var raw string
		if err := websocket.Message.Receive(conn, &raw); err != nil {
			dropWatchClient(server, nil)
			return
		}

		if strings.HasPrefix(raw, watchAddPrefix) || strings.HasPrefix(raw, watchRemovePrefix) {
			continue
		}

		// Extract vaultID and clientID from message format "vaultID:clientID:filename"
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) != 3 {
			continue
		}
		vaultID := parts[0]
		clientID := parts[1]
		name := strings.TrimSpace(parts[2])
		if name == "" {
			continue
		}

		sendToSubscribers(server, name, vaultID, clientID)
	}
}

func safeSend(ch chan string, value string) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()

	ch <- value
	return true
}

func getOrCreateWatchClient(server string) (*syncClient, error) {
	if server == "" {
		return nil, core.Error(core.ConfigError, "server is empty")
	}

	// Use server as cache key - all vault instances share one connection to each server
	cacheKey := server

	watchClientsMu.Lock()
	client := watchClients[cacheKey]
	watchClientsMu.Unlock()

	if client != nil {
		return client, nil
	}

	u, err := url.Parse(server)
	if err != nil {
		return nil, core.Error(core.ParseError, "cannot parse server URL", err)
	}
	if u.Scheme != "wss" && u.Scheme != "ws" {
		return nil, core.Error(core.NetError, "server must be a wss or ws URL")
	}
	if u.Host == "" {
		return nil, core.Error(core.NetError, "server host is empty")
	}
	wsURL := u.String()
	origin := "http://" + u.Host
	if u.Scheme == "wss" {
		origin = "https://" + u.Host
	}

	core.Info("connecting to sync relay server: %s (origin: %s)", wsURL, origin)
	conn, err := websocket.Dial(wsURL, "", origin)
	if err != nil {
		core.LogError("failed to connect to sync relay server %s", wsURL, err)
		return nil, core.Error(core.NetError, "cannot connect to sync relay server", err)
	}

	client = &syncClient{
		conn:        conn,
		subscribers: make(map[string]map[string]*Vault),
	}

	watchClientsMu.Lock()
	watchClients[cacheKey] = client
	watchClientsMu.Unlock()

	go readWatchEvents(conn, cacheKey)
	return client, nil
}

func sendToSubscribers(server string, name string, vaultID string, senderClientID string) {
	core.Start("server %s, name %s, vaultID %s, senderClientID %s", server, name, vaultID, senderClientID)
	watchClientsMu.Lock()
	client := watchClients[server]
	watchClientsMu.Unlock()
	if client == nil {
		core.End("no watch client for server %s", server)
		return
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	subs := client.subscribers[vaultID]
	if subs == nil {
		core.End("no subscribers for vaultID %s", vaultID)
		return
	}
	for clientID, v := range subs {
		if clientID != senderClientID && v.syncRelayCh != nil {
			go safeSend(v.syncRelayCh, name)
		}
	}
	core.End("sent to %d subscribers", len(subs))
}

func dropWatchClient(server string, client *syncClient) {
	watchClientsMu.Lock()
	current := watchClients[server]
	if client == nil || current == client {
		delete(watchClients, server)
	}
	watchClientsMu.Unlock()

	if current != nil && (client == nil || current == client) {
		_ = current.conn.Close()
	}
}
