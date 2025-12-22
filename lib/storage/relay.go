package storage

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
)

type Relay struct {
	inner Store
}

type RelayConfig struct {
	URL       string             `json:"url"`
	PrivateID security.PrivateID `json:"privateId"`
}

type relayRequest struct {
	PublicIDHash []byte `json:"publicIDHash"`
	Timestamp    string `json:"timestamp"`
	Signature    []byte `json:"signature"`
}

// OpenRelay creates a new Relay storage that forwards all operations to an inner storage
// A relay is a web service that returns a storage endpoint given a public ID. The endpoint
// configuration is encrypted with the private ID corresponding to the public ID.
// The request to the relay service must be signed with the private ID to prove ownership.
func OpenRelay(id string, c RelayConfig) (Store, error) {
	// Call a POST to c.Url with the RelayRequest containing the PublicIDHash, Timestamp and Signature
	// The response contains the storage endpoint configuration encrypted with the private ID

	hash := core.BigHash(c.PrivateID.PublicIDMust().Bytes())

	timestamp := core.Now().UTC().Format(time.RFC3339)
	signature, err := security.Sign(c.PrivateID, []byte(timestamp))
	if err != nil {
		return nil, core.Errorw("cannot sign relay request: %v", err)
	}

	req := relayRequest{
		PublicIDHash: hash,
		Timestamp:    timestamp,
		Signature:    signature,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, core.Errorw("cannot marshal relay request: %v", err)
	}

	resp, err := http.Post(c.URL, "application/json", core.NewBytesReader(body))
	if err != nil {
		return nil, core.Errorw("cannot call relay service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, core.Errorw("relay service returned status %d", resp.StatusCode)
	}

	encryptedConfig, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.Errorw("cannot read relay service response: %v", err)
	}

	decryptedConfigBytes, err := security.EcDecrypt(c.PrivateID, encryptedConfig)
	if err != nil {
		return nil, core.Errorw("cannot decrypt relay service response: %v", err)
	}

	var storeConfig StoreConfig
	err = json.Unmarshal(decryptedConfigBytes, &storeConfig)
	if err != nil {
		return nil, core.Errorw("cannot unmarshal store config from relay response: %v", err)
	}

	innerStore, err := Open(storeConfig)
	if err != nil {
		return nil, core.Errorw("cannot open inner store from relay config: %v", err)
	}

	return &Relay{inner: innerStore}, nil
}

func (r *Relay) ReadDir(name string, filter Filter) ([]fs.FileInfo, error) {
	return r.inner.ReadDir(name, filter)
}

// Read reads data from a file into a writer
func (r *Relay) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	return r.inner.Read(name, rang, dest, progress)
}

// Write writes data to a file name. An existing file is overwritten
func (r *Relay) Write(name string, source io.ReadSeeker, progress chan int64) error {
	return r.inner.Write(name, source, progress)
}

// Stat provides statistics about a file
func (r *Relay) Stat(name string) (os.FileInfo, error) {
	return r.inner.Stat(name)
}

// Delete deletes a file
func (r *Relay) Delete(name string) error {
	return r.inner.Delete(name)
}

// ID returns an identifier for the store, typically the URL without credentials information and other parameters
func (r *Relay) ID() string {
	return r.inner.ID()
}

// Close releases resources
func (r *Relay) Close() error {
	return r.inner.Close()
}

// String returns a human-readable representation of the storer (e.g. sftp://user@host.cc/path)
func (r *Relay) String() string {
	return r.inner.String()
}

// Describe returns cost details of the store
func (r *Relay) Describe() Description {
	return r.inner.Describe()
}
