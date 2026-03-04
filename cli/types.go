//go:build !js

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stregato/bao/lib/replica"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/stregato/bao/lib/vault"
	yaml "gopkg.in/yaml.v2"
)

type Session struct {
	PrivateID      string `yaml:"privateId"`
	CreatorPublic  string `yaml:"creatorPublicId"`
	StoreConfig    string `yaml:"storeConfigPath"`
	VaultDBPath    string `yaml:"vaultDbPath"`
	ReplicaDBPath  string `yaml:"replicaDbPath"`
	ReplicaDir     string `yaml:"replicaDir"`
	ReplicaDDLPath string `yaml:"replicaDdlPath"`
}

type App struct {
	sessionPath string
	session     Session

	store     store.Store
	db        *sqlx.DB
	v         *vault.Vault
	replicaDB *sqlx.DB
	replica   *replica.Replica
	cwd       string
}

func defaultSessionPath() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ".bao-cli-session.yaml"
	}
	return filepath.Join(h, ".bao", "cli-session.yaml")
}

func newApp() *App {
	a := &App{
		sessionPath: defaultSessionPath(),
		session: Session{
			VaultDBPath:   "myapp/main.sqlite",
			ReplicaDBPath: "myapp/replica.sqlite",
			ReplicaDir:    "replica",
		},
	}
	_ = a.loadSession()
	return a
}

func (a *App) loadSession() error {
	b, err := os.ReadFile(a.sessionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var s Session
	if err := yaml.Unmarshal(b, &s); err != nil {
		return err
	}
	if s.VaultDBPath == "" {
		s.VaultDBPath = "myapp/main.sqlite"
	}
	if s.ReplicaDBPath == "" {
		s.ReplicaDBPath = "myapp/replica.sqlite"
	}
	if s.ReplicaDir == "" {
		s.ReplicaDir = "replica"
	}
	a.session = s
	return nil
}

func (a *App) saveSession() error {
	if err := os.MkdirAll(filepath.Dir(a.sessionPath), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(a.session)
	if err != nil {
		return err
	}
	return os.WriteFile(a.sessionPath, b, 0o600)
}

func (a *App) closeAll() {
	if a.replicaDB != nil {
		_ = a.replicaDB.Close()
		a.replicaDB = nil
	}
	a.replica = nil
	if a.v != nil {
		_ = a.v.Close()
		a.v = nil
	}
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	if a.store != nil {
		_ = a.store.Close()
		a.store = nil
	}
}

func (a *App) mustVault() error {
	if a.v == nil {
		return fmt.Errorf("vault not opened (run: open ...)")
	}
	return nil
}

func (a *App) mustReplica() error {
	if a.replica == nil {
		return fmt.Errorf("replica not opened (run: replica-open ...)")
	}
	return nil
}

func (a *App) printIdentitySummary() {
	privateID := strings.TrimSpace(a.session.PrivateID)
	if privateID == "" {
		fmt.Println("Private ID: <not set>")
		fmt.Println("Public ID : <not set>")
		return
	}
	pub, err := security.PrivateID(privateID).PublicID()
	if err != nil {
		fmt.Printf("Private ID: %s\n", privateID)
		fmt.Printf("Public ID : <error: %v>\n", err)
		return
	}
	fmt.Printf("Private ID: %s\n", privateID)
	fmt.Printf("Public ID : %s\n", pub)
}

func parseStoreConfig(filePath string) (store.StoreConfig, error) {
	var cfg store.StoreConfig
	b, err := os.ReadFile(filePath)
	if err != nil {
		return cfg, err
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return cfg, fmt.Errorf("empty store config")
	}
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
		if err := json.Unmarshal([]byte(t), &cfg); err != nil {
			return cfg, err
		}
	} else {
		if err := yaml.Unmarshal([]byte(t), &cfg); err != nil {
			return cfg, err
		}
	}
	if cfg.Id == "" {
		cfg.Id = cfg.Type
	}
	return cfg, nil
}

func ensureParentDir(filePath string) error {
	d := filepath.Dir(filePath)
	if d == "." || d == "" {
		return nil
	}
	return os.MkdirAll(d, 0o755)
}

func normalizeVaultPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	if p == "." {
		return ""
	}
	return p
}

func joinVaultPath(base, child string) string {
	b := normalizeVaultPath(base)
	c := normalizeVaultPath(child)
	if b == "" {
		return c
	}
	if c == "" {
		return b
	}
	return b + "/" + c
}

func attrsPreview(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if !utf8.Valid(b) {
		return fmt.Sprintf("<binary:%d>", len(b))
	}
	s := string(b)
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return fmt.Sprintf("<binary:%d>", len(b))
		}
	}
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}
