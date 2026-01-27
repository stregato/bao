package vault

import "github.com/stregato/bao/lib/security"

type KeyPolicy interface {
	GetKey(path string, id uint64) (key security.AESKey, err error)
	NewKey(path string) (id uint64, key security.AESKey, err error)
}

type AllPolicy struct{}
	
func (p *AllPolicy) GetKey(path string, id uint64) (key security.AESKey, err error) {
	return nil, nil
}
func (p *AllPolicy) NewKey(path string) (id uint64, key security.AESKey, err error) {
	return 0, nil, nil
}

type HomePolicy struct {
	Vault *Vault
}

func (p *HomePolicy) GetKey(path string, id uint64) (key security.AESKey, err error) {
	return p.Vault.getKey(id)
}
