package bao

import "gopkg.in/yaml.v2"

func (s *Bao) MarshalYAML() (any, error) {
	groups, err := s.ListGroups()
	if err != nil {
		return nil, err
	}
	var accesses = map[Group]Accesses{}

	for _, group := range groups {
		users, err := s.GetUsers(group)
		if err != nil {
			return nil, err
		}
		accesses[group] = users
	}

	return map[string]any{
		"id":       s.Id,
		"url":      s.Url,
		"storeId":  s.Id,
		"author":   s.Author,
		"config":   s.Config,
		"accesses": accesses,
	}, nil
}

func (s *Bao) String() string {
	data, _ := yaml.Marshal(s)
	return string(data)
}
