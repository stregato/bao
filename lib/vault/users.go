package vault

import (
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

func (v *Vault) removeUser(userID security.PublicID) error {
	core.Start("removing user %s from %s", userID, v.ID)
	_, err := v.DB.Exec("REMOVE_USER", sqlx.Args{"vault": v.ID, "userId": userID})
	if err != nil {
		return core.Errorw("cannot remove user %s from %s", userID, v.ID, err)
	}
	core.Info("removed user %s from %s", userID, v.ID)
	core.End("")
	return nil
}

func (v *Vault) setUser(userID security.PublicID, access Access) error {
	core.Start("setting user %s with access %s in %s", userID, AccessLabels[access], v.ID)

	shortID := userID.Hash()
	_, err := v.DB.Exec("SET_USER", sqlx.Args{"vault": v.ID, "userId": userID, "shortId": shortID, "access": access})
	if err != nil {
		return core.Errorw("cannot set user %s in %s", userID, v.ID, err)
	}

	core.End("set user %s with access %s in %s", userID, AccessLabels[access], v.ID)
	return nil
}

// func (v *Vault) getUserIDbyShortID(shortID uint64) (security.PublicID, error) {
// 	var userID security.PublicID

// 	core.Start("getting user by short ID %d", shortID)
// 	err := v.DB.QueryRow("GET_USER_ID_BY_SHORT_ID", sqlx.Args{
// 		"vault":   v.ID,
// 		"shortId": shortID,
// 	}, &userID)

// 	return userID, err
// }
