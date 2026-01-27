package replica

import (
	_ "embed"
	"sync"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/vault"
)

type transaction struct {
	tx      *sqlx.TxX
	Updates []Update     // Updates is a list of Update encoded in msgpack and encrypted
	Version float32      // Version is the highest version of the updates
	Id      vault.FileId // Id is the id of the transaction
	Tm      time.Time    // Tm is the time of the transaction
}

type Replica struct {
	vault *vault.Vault
	//	lastIds     map[uint64]struct{} // lastIds is a list of last ids for each table
	lastId      int64        // lastId is the last id used for the transaction
	db          *sqlx.DB     // db is the database connection for the layer
	execLock    sync.Mutex   // execLock is a lock for executing SQL statements
	queryLock   sync.Mutex   // queryLock is a lock for executing SQL queries
	transaction *transaction // transaction is the current transaction for the layer
}

const sqlLayerDir = "replica"

func Open(vault *vault.Vault, db *sqlx.DB) (*Replica, error) {
	core.Start("vault %s", vault.ID)

	lastId, err := readLastTransactionsId(vault.DB, vault.String())
	if err != nil {
		return nil, core.Errorw("cannot read last transaction id", err)
	}

	core.End("")

	return &Replica{vault: vault, lastId: lastId, db: db, execLock: sync.Mutex{}, queryLock: sync.Mutex{}, transaction: nil}, nil
}

func readLastTransactionsId(db *sqlx.DB, vaultID string) (lastId int64, err error) {
	core.Start("")
	err = db.QueryRow("GET_LAST_TRANSACTION_METADATA_ID", sqlx.Args{"vault": vaultID}, &lastId)
	if err == sqlx.ErrNoRows {
		core.End("no transaction id")
		return 0, nil
	}
	if err != nil {
		return 0, core.Errorw("cannot read last transaction id", err)
	}
	core.End("transaction id %d", lastId)
	return lastId, nil
}
