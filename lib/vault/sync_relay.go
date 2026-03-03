package vault

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/stregato/bao/lib/core"
	"golang.org/x/net/websocket"
)

const watchQueueSize = 1024

func (v *Vault) relayClientID() string {
	return fmt.Sprintf("%d-%p", os.Getpid(), v)
}

func (v *Vault) relayWatchFolders() []string {
	return []string{
		v.dataRoot(),
		v.blockChainRoot(),
	}
}

func (v *Vault) startSyncRelay() error {
	if v.syncRelayCh != nil {
		core.Info("sync relay already running for vault %s", v.ID)
		return nil
	}
	if v.Config.SyncRelay == "" {
		core.Info("sync relay disabled for vault %s: empty config", v.ID)
		return nil
	}

	// Open or reuse the websocket connection to the sync relay server
	client, err := getOrCreateWatchClient(v.Config.SyncRelay)
	if err != nil {
		return err
	}

	client.mu.Lock()
	if _, exists := client.subscribers[v.ID]; !exists {
		for _, folder := range v.relayWatchFolders() {
			if err := websocket.Message.Send(client.conn, watchAddPrefix+folder); err != nil {
				client.mu.Unlock()
				return core.Error(core.NetError, "cannot send watch add message", err)
			}
		}
		client.subscribers[v.ID] = make(map[string]*Vault)
	}
	instanceID := v.relayClientID()
	client.subscribers[v.ID][instanceID] = v
	client.mu.Unlock()
	v.syncRelayCh = make(chan string, watchQueueSize)
	core.Info("sync relay started for vault %s with clientID %s on %s", v.ID, instanceID, v.Config.SyncRelay)

	go v.relayLoop()
	return nil
}

func (v *Vault) relayLoop() {
	defer v.cleanupSyncRelay()

	v.Sync()
	blockchainPrefix := v.blockChainRoot()
	dataPrefix := v.dataRoot()

	for name := range v.syncRelayCh {
		core.Info("relay loop received event for vault %s: %s", v.ID, name)
		switch {
		case strings.HasPrefix(name, blockchainPrefix+"/"):
			core.Info("relay loop triggering blockchain sync for vault %s due to %s", v.ID, name)
			v.syncBlockChain(true)
		case strings.HasPrefix(name, dataPrefix+"/"):
			core.Info("relay loop triggering data sync for vault %s due to %s", v.ID, name)
			now := core.Now()
			dir, name := path.Split(name)
			dir = path.Clean(dir)
			file, synced, deferred, err := v.syncronizeFile(dir, name)
			if err != nil {
				core.Error("failed to sync file %s/%s/%s: %v", v.ID, dir, name, err)
			} else if deferred {
				v.scheduleDeferredRelayRetry(dir, name, file.Size)
			} else if synced {
				v.lastSyncAt = now
			}
		}
	}
}

func relayDeferredRetryDelay(size int64) time.Duration {
	switch {
	case size > 100*1024*1024:
		return 8 * time.Second
	case size > 20*1024*1024:
		return 5 * time.Second
	default:
		return 2 * time.Second
	}
}

func (v *Vault) scheduleDeferredRelayRetry(storeDir, storeName string, size int64) {
	key := path.Join(storeDir, storeName)

	v.relayRetryMu.Lock()
	if _, exists := v.relayRetry[key]; exists {
		v.relayRetryMu.Unlock()
		return
	}
	v.relayRetry[key] = struct{}{}
	v.relayRetryMu.Unlock()

	delay := relayDeferredRetryDelay(size)
	core.Info("scheduling deferred relay retry for %s in %s", key, delay)
	go func() {
		time.Sleep(delay)
		defer func() {
			v.relayRetryMu.Lock()
			delete(v.relayRetry, key)
			v.relayRetryMu.Unlock()
		}()

		if v.syncRelayCh == nil {
			return
		}

		file, synced, deferred, err := v.syncronizeFile(storeDir, storeName)
		if err != nil {
			core.Error("deferred relay retry failed for %s: %v", key, err)
			return
		}
		if deferred {
			core.Info("deferred relay retry still waiting for body readiness: %s", key)
			return
		}
		if synced {
			v.lastSyncAt = core.Now()
			core.Info("deferred relay retry imported file %s size %d", key, file.Size)
		}
	}()
}

func (v *Vault) cleanupSyncRelay() {
	client, err := getOrCreateWatchClient(v.Config.SyncRelay)
	if err != nil {
		return
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	instanceID := v.relayClientID()
	subs := client.subscribers[v.ID]
	if subs == nil {
		return
	}

	// Remove this vault instance from the subscriber list
	delete(subs, instanceID)

	// If no more subscribers for this vault, clean up
	if len(subs) == 0 {
		delete(client.subscribers, v.ID)

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
	instanceID := v.relayClientID()
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
			core.Info("sync relay received malformed event on %s: %q", server, raw)
			continue
		}
		vaultID := parts[0]
		clientID := parts[1]
		name := strings.TrimSpace(parts[2])
		if name == "" {
			core.Info("sync relay received empty filename event on %s for vault %s", server, vaultID)
			continue
		}
		core.Info("sync relay received event on %s: vault=%s sender=%s name=%s", server, vaultID, clientID, name)

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
	delivered := 0
	skippedSender := 0
	inactive := 0
	for clientID, v := range subs {
		if clientID == senderClientID {
			skippedSender++
			continue
		}
		if v.syncRelayCh != nil {
			go safeSend(v.syncRelayCh, name)
			delivered++
		} else {
			inactive++
		}
	}
	core.End("subscribers=%d delivered=%d skippedSender=%d inactive=%d", len(subs), delivered, skippedSender, inactive)
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
