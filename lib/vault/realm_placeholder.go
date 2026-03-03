package vault

import (
	"path"
)

func (v *Vault) dataRoot() string {
	return path.Join(DataFolder)
}

func (v *Vault) blockChainRoot() string {
	return path.Join(BlockChainFolder)
}
