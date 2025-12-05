package bao

import (
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

func (s *Bao) SetAttribute(options IOOption, name, value string) error {
	core.Start("name %s, value %s", name, value)
	change := &AddAttribute{
		Name:  name,
		Value: value,
	}
	bc, err := marshalChange(change)
	if err != nil {
		return core.Errorw("cannot marshal attribute change %s for user %s", name, s.UserPublicId, err)
	}
	err = s.stageBlockChange(bc)
	if err != nil {
		return core.Errorw("cannot stage blockchain change for attribute %s", name, err)
	}

	switch {
	case options&AsyncOperation != 0:
		go s.syncBlockChain()
	case options&ScheduledOperation != 0:
		// Do nothing, sync will be done later
	default:
		err = s.syncBlockChain()
		if err != nil {
			return core.Errorw("cannot synchronize blockchain for attribute change", err)
		}
	}

	core.End("successfully added attribute %s for public id %s", name, s.UserPublicId)
	return nil
}

func (s *Bao) GetAttribute(name string, author security.PublicID) (string, error) {
	core.Start("getting attribute %s for id %s", name, author)
	var value string
	err := s.DB.QueryRow("GET_ATTRIBUTE", sqlx.Args{
		"store": s.Id,
		"name":  name,
		"id":    author,
	}, &value)
	if err != nil {
		return "", core.Errorw("cannot get attribute %s for id %s", name, author, err)
	}
	core.End("successfully got attribute %s for id %s: %s", name, author, value)
	return value, nil
}

func (s *Bao) GetAttributes(author security.PublicID) (map[string]string, error) {
	core.Start("getting attributes for id %s", author)
	attrs := make(map[string]string)
	rows, err := s.DB.Query("GET_ATTRIBUTES", sqlx.Args{
		"store": s.Id,
		"id":    author,
	})
	if err != nil {
		return nil, core.Errorw("cannot list attributes for id %s", author, err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, core.Errorw("cannot scan attribute for id %s", author, err)
		}
		attrs[name] = value
	}
	core.End("successfully listed attributes for id %s: %v", author, attrs)
	return attrs, nil
}
