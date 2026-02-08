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

func (v *Vault) startSyncRelay() error {
	if v.Config.SyncRelay == "" {
		if v.stopSyncRelay != nil {
			close(v.stopSyncRelay)
			v.stopSyncRelay = nil
		}
		return nil // No sync relay configured, nothing to do
	}
	if v.stopSyncRelay != nil {
		return nil // Sync relay already running
	}

	// Use vault pointer as unique instance ID to ensure each vault instance gets its own connection
	blockchainCh, err := v.watchFolder(path.Join(v.Realm.String(), BlockChainFolder))
	if err != nil {
		return core.Error(core.GenericError, "cannot watch blockchain folder in sync relay", err)
	}
	dataCh, err := v.watchFolder(path.Join(v.Realm.String(), DataFolder))
	if err != nil {
		return core.Error(core.GenericError, "cannot watch data folder in sync relay", err)
	}

	go func() {
		v.Sync()
		for {
			select {
			case <-v.stopSyncRelay:
				close(blockchainCh)
				close(dataCh)
				return
			case <-blockchainCh:
				v.syncBlockChain()
			case name := <-dataCh:
				now := core.Now()
				dir, name := path.Split(name)
				dir = path.Clean(dir)
				_, err = v.syncronizeFile(dir, name)
				if err != nil {
					core.Error("failed to sync file %s/%s/%s: %v", v.ID, dir, name, err)
				} else {
					v.lastSyncAt = now
				}
			}
		}
	}()
	v.stopSyncRelay = make(chan struct{})
	return nil
}

const watchAddPrefix = "+"
const watchRemovePrefix = "-"

type syncClient struct {
	conn        *websocket.Conn
	folders     map[string]struct{}
	subscribers map[chan string]struct {
		vaultID  string
		folder   string
		clientID string
	}
	mu sync.Mutex
}

var watchClientsMu sync.Mutex
var watchClients = map[string]*syncClient{}

func (v *Vault) watchFolder(folder string) (chan string, error) {
	core.Start("folder %s", folder)
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return nil, core.Error(core.GenericError, "folder is empty")
	}

	client, err := getOrCreateWatchClient(v.Config.SyncRelay)
	if err != nil {
		return nil, err
	}

	instanceID := fmt.Sprintf("%p", v)
	onChange := make(chan string)
	client.mu.Lock()
	if _, exists := client.folders[folder]; !exists {
		client.folders[folder] = struct{}{}
	}
	client.subscribers[onChange] = struct {
		vaultID  string
		folder   string
		clientID string
	}{vaultID: v.ID, folder: folder, clientID: instanceID}
	client.mu.Unlock()

	if err := websocket.Message.Send(client.conn, watchAddPrefix+folder); err != nil {
		return nil, core.Error(core.NetError, "cannot send watch add message", err)
	}
	core.End("onChange=%p", onChange)
	return onChange, nil
}

func (v *Vault) notifyChange(filename string) error {
	core.Start("filename %s", filename)
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
		conn:    conn,
		folders: map[string]struct{}{},
		subscribers: map[chan string]struct {
			vaultID  string
			folder   string
			clientID string
		}{},
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

	for ch, sub := range client.subscribers {
		// Filter by vaultID - only send to subscribers of the same vault
		if sub.vaultID != vaultID {
			continue
		}
		// Skip sending back to the client that sent the notification
		if sub.clientID == senderClientID {
			continue
		}
		// Only send if the filename matches the subscribed folder
		if strings.HasPrefix(name, sub.folder+"/") || name == sub.folder {
			if !safeSend(ch, name) {
				delete(client.subscribers, ch)
			}
		}
	}
	core.End("sent to %d subscribers", len(client.subscribers))
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
