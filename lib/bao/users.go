package bao

import (
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

func (s *Bao) removeUser(group Group, publicId security.PublicID) error {
	core.Start("removing user %s from group %s", publicId, group)
	_, err := s.DB.Exec("REMOVE_USER", sqlx.Args{"store": s.Id, "group": group, "publicId": publicId})
	if err != nil {
		return core.Errorw("cannot remove user %s from group %s", publicId, group, err)
	}
	core.Info("removed user %s from group %s", publicId, group)
	core.End("")
	return nil
}

func (s *Bao) setUser(group Group, publicId security.PublicID, access Access) error {
	core.Start("setting user %s with access %s for group %s", publicId, AccessLabels[access], group)

	var id int64
	err := s.DB.QueryRow("GET_ID", sqlx.Args{"publicId": publicId}, &id)
	if err == sqlx.ErrNoRows {
		res, err := s.DB.Exec("SET_ID", sqlx.Args{"publicId": publicId})
		if err != nil {
			return core.Errorw("cannot set id for user %s", publicId, err)
		}

		id, err = res.LastInsertId()
		if err != nil {
			return core.Errorw("cannot get last insert id for user %s", publicId, err)
		}
	}

	_, err = s.DB.Exec("SET_USER", sqlx.Args{"store": s.Id, "group": group, "id": id, "access": access})
	if err != nil {
		return core.Errorw("cannot set user %s for group %s", publicId, group, err)
	}

	var keyId = core.Int64Hash(publicId.Bytes()) // Generate a keyId for the user, ensuring bit 63 is unset and has bit 62 set
	err = s.setKeyToDB(keyId, Group(publicId), nil)
	if err != nil {
		return core.Errorw("cannot set key for user %s in group %s", publicId, group, err)
	}
	core.End("set user %s with access %s for group %s", publicId, AccessLabels[access], group)
	return nil
}
