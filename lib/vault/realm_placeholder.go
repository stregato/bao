package vault

import (
	"path"
)

// TODO(realm-removal): Replace this compatibility placeholder with per-base policy resolution.
func (v *Vault) legacyRealm() Realm {
	return Users
}

func (v *Vault) dataRoot() string {
	// TODO(realm-removal): Add per-base routing for data directories.
	return path.Join(DataFolder)
}

func (v *Vault) blockChainRoot() string {
	// TODO(realm-removal): Add per-base routing for blockchain directories.
	return path.Join(BlockChainFolder)
}
