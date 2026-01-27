package vault

import "gopkg.in/yaml.v2"

func (v *Vault) MarshalYAML() (any, error) {
	accesses, err := v.GetAccesses()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"id":       v.ID,
		"storeId":  v.ID,
		"author":   v.Author,
		"config":   v.Config,
		"accesses": accesses,
	}, nil
}

func (v *Vault) String() string {
	data, _ := yaml.Marshal(v)
	return string(data)
}
