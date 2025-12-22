package ql

import (
	_ "embed"
	"sync"
	"time"

	"github.com/stregato/bao/lib/bao"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

type transaction struct {
	tx      *sqlx.TxX
	Updates []Update   // Updates is a list of Update encoded in msgpack and encrypted
	Version float32    // Version is the highest version of the updates
	Id      bao.FileId // Id is the id of the transaction
	Tm      time.Time  // Tm is the time of the transaction
}

type BaoQL struct {
	s *bao.Bao
	//	lastIds     map[uint64]struct{} // lastIds is a list of last ids for each table
	lastId      int64        // lastId is the last id used for the transaction
	db          *sqlx.DB     // db is the database connection for the layer
	execLock    sync.Mutex   // execLock is a lock for executing SQL statements
	queryLock   sync.Mutex   // queryLock is a lock for executing SQL queries
	group       bao.Group    // group is the group of the layer, used to filter transactions
	transaction *transaction // transaction is the current transaction for the layer
}

const sqlLayerDir = "bao_ql"

func SQL(s *bao.Bao, group bao.Group, db *sqlx.DB) (*BaoQL, error) {
	core.Start("vault %s, group %s", s.Id, group)

	lastId, err := readLastTransactionsId(s.DB, s.String(), group)
	if err != nil {
		return nil, core.Errorw("cannot read last transaction id", err)
	}

	core.End("")

	return &BaoQL{s, lastId, db, sync.Mutex{}, sync.Mutex{}, group, nil}, nil
}

func readLastTransactionsId(db *sqlx.DB, store string, group bao.Group) (lastId int64, err error) {
	core.Start("group %s", group)
	err = db.QueryRow("GET_LAST_TRANSACTION_METADATA_ID", sqlx.Args{"store": store, "group": group}, &lastId)
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
