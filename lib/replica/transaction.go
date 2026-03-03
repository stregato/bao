package replica

import (
	"database/sql"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/vault"
	"github.com/vmihailenco/msgpack/v5"
)

type Update struct {
	Key  string
	Args sqlx.Args
}

func (ds *Replica) beginTransaction() error {
	core.Start("")
	tx, err := ds.db.Begin()
	if err != nil {
		return core.Error(core.GenericError, "cannot start transaction", err)
	}
	ds.transaction = &transaction{
		tx:      tx,
		Updates: make([]Update, 0),
		Version: 0.0, // initial version
		//		Id:      core.SnowID(), // generate a new transaction ID
		Tm: core.Now(), // set the current time
	}

	core.End("")
	return nil
}

func (ds *Replica) Exec(key string, args sqlx.Args) (sql.Result, error) {
	core.Start("key %s, args %v", key, args)
	ds.execLock.Lock()
	defer ds.execLock.Unlock()

	if ds.transaction == nil {
		err := ds.beginTransaction()
		if err != nil {
			return nil, err
		}
	}

	res, err := ds.transaction.tx.Exec(key, args)
	if err != nil {
		return nil, err
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		rowsAffected, _ := res.RowsAffected()
		core.End("exec completed: row affected %d, key %s", rowsAffected, key)
	}

	ds.transaction.Updates = append(ds.transaction.Updates, Update{key, args})
	return res, nil
}

// Sync writes the current transaction to the store and reads all transactions that are not yet processed.
// It executes the updates in the transactions and commits them to the database.
// It returns the number of updates processed or an error if something goes wrong.
func (ds *Replica) Sync(dests ...security.PublicID) (int, error) {
	core.Start("")
	now := time.Now()
	ds.syncLock.Lock()
	defer ds.syncLock.Unlock()

	readDir := replicaDir

	// List and read unread transactions only if needed
	var transactions []transaction
	ls, err := ds.listUnreadTransactions(readDir)
	if err != nil {
		return 0, core.Error(core.DbError, "cannot get transactions files", err)
	}

	transactions, err = ds.readTransactionFiles(ls)
	if err != nil {
		return 0, core.Error(core.GenericError, "cannot read transactions", err)
	}

	// Add current transaction to replica dir.
	// If destinations are provided, write one EC-encrypted copy per recipient.
	transactions, err = ds.addCurrentTransactionToAll(replicaDir, dests, transactions)
	if err != nil {
		return 0, core.Error(core.DbError, "cannot write current transaction in replica", err)
	}

	// Process all transactions
	updates, err := ds.processTransactions(transactions)
	if err != nil {
		return 0, core.Error(core.GenericError, "cannot process transactions", err)
	}

	core.End("elapsed %s", time.Since(now))
	return len(updates), nil
}

func (ds *Replica) addCurrentTransactionToAll(dir string, dests []security.PublicID, transactions []transaction) ([]transaction, error) {
	core.Start("transaction %p", ds.transaction)
	now := time.Now()
	ds.execLock.Lock()
	if ds.transaction == nil {
		ds.execLock.Unlock()
		core.End("ds.transaction == nil")
		return transactions, nil // no current transaction to add, return the original list
	}

	t := *ds.transaction
	ds.transaction = nil // reset the current transaction to nil before writing
	ds.execLock.Unlock()

	err := t.tx.Rollback() // rollback the transaction to ensure it is not committed yet
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot rollback transaction", err)
	}

	// Marshal and compress once
	attrs, err := msgpack.Marshal(t)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot marshal transaction %d", t.Id, err)
	}
	attrs, err = core.GzipCompress(attrs)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot compress transaction %d", t.Id, err)
	}

	// Write transaction file(s)
	name := strconv.FormatUint(core.SnowID(), 16) // base logical tx name
	var maxWrittenID vault.FileId
	if len(dests) == 0 {
		file, err := ds.vault.Write(path.Join(dir, name), "", attrs, vault.IOOption{})
		if err != nil {
			return nil, core.Error(core.DbError, "cannot write transaction %d to %s", t.Id, dir, err)
		}
		maxWrittenID = file.Id
	} else {
		for _, dest := range dests {
			txName := fmt.Sprintf("%s-%x,ec=%s", name, dest.Hash(), dest)
			file, err := ds.vault.Write(path.Join(dir, txName), "", attrs, vault.IOOption{})
			if err != nil {
				return nil, core.Error(core.DbError, "cannot write transaction %d to %s for recipient %s", t.Id, dir, dest, err)
			}
			if file.Id > maxWrittenID {
				maxWrittenID = file.Id
			}
		}
	}
	t.Id = maxWrittenID

	core.End("name %s, elapsed %s, updates %d, recipients %d", name, core.Since(now), len(t.Updates), len(dests))
	return append(transactions, t), nil
}

func (ds *Replica) Cancel() error {
	core.Start("")
	ds.execLock.Lock()
	defer ds.execLock.Unlock()
	if ds.transaction == nil {
		core.End("no transaction to cancel for replica in vault %s", ds.vault.ID)
		return nil
	}

	err := ds.transaction.tx.Rollback()
	if err != nil {
		return core.Error(core.GenericError, "cannot rollback transaction", err)
	}
	ds.transaction = nil

	core.End("")
	return nil
}

func (ds *Replica) listUnreadTransactions(dir string) (ls []vault.File, err error) {
	core.Start("")
	files, err := ds.vault.ReadDir(dir, time.Time{}, ds.lastId, 0)
	if err != nil {
		if err == sqlx.ErrNoRows {
			core.End("no transaction files")
			return nil, nil // no files found, nothing to process
		}
		return nil, core.Error(core.FileError, "cannot read files in replica", err)
	}

	slices.SortFunc(files, func(a, b vault.File) int {
		return strings.Compare(a.Name, b.Name) // sort by file name
	})

	core.End("%d transaction files", len(files))
	return files, nil
}

func (ds *Replica) readTransactionFiles(ls []vault.File) (transactions []transaction, err error) {
	core.Start("reading %d transaction files", len(ls))
	n_updates := 0
	for _, fi := range ls {
		transaction, err := ds.readTransaction(fi)
		if err != nil {
			core.Error(core.GenericError, "cannot read transaction in replica", err)
			continue
		}
		transaction.Id = fi.Id // set the transaction Id from the file Id
		transactions = append(transactions, transaction)
		n_updates += len(transaction.Updates)
	}
	core.End("read %d transactions with total of %d updates", len(transactions), n_updates)
	return transactions, nil
}

func (ds *Replica) processTransactions(t []transaction) (updates []Update, err error) {
	core.Start("%d transactions", len(t))
	updates = make([]Update, 0)
	for _, transaction := range t {
		err = ds.processTransaction(transaction)
		if err != nil {
			return nil, core.Error(core.GenericError, "cannot process transaction %d", transaction.Id, err)
		}
		updates = append(updates, transaction.Updates...)
		if transaction.Id > ds.lastId {
			ds.lastId = transaction.Id // keep high-watermark over all processed transactions
		}
	}
	core.End("%d updates, lastId %d", len(updates), ds.lastId)
	return updates, nil
}

func (ds *Replica) readTransaction(fi vault.File) (transaction, error) {
	core.Start("file %s", fi.Name)
	var t transaction

	attrs, err := core.GzipDecompress(fi.Attrs)
	if err != nil {
		return transaction{}, core.Error(core.EncodeError, "cannot decompress transaction %s", fi.Name, err)
	}

	reader := core.NewBytesReader(attrs)
	err = msgpack.NewDecoder(reader).Decode(&t)
	if err != nil {
		return transaction{}, core.Error(core.ParseError, "cannot unmarshal transaction %s", fi.Name, err)
	}

	core.End("%d updates", len(t.Updates))
	return t, nil
}

func (ds *Replica) processTransaction(t transaction) error {
	core.Start("id %d, %d updates", t.Id, len(t.Updates))

	ds.queryLock.Lock()
	for _, u := range t.Updates {
		_, err := ds.db.Exec(u.Key, u.Args)
		if err != nil {
			ds.queryLock.Unlock()
			return core.Error(core.GenericError, "cannot execute transaction %s", u.Key, err)
		}
	}
	ds.queryLock.Unlock()

	if t.Id > ds.lastId {
		ds.lastId = t.Id
	}
	_, err := ds.vault.DB.Exec("INSERT_TRANSACTION_METADATA", sqlx.Args{"vault": ds.vault.ID, "id": t.Id,
		"tm": t.Tm, "success": true})
	if err != nil {
		return core.Error(core.DbError, "cannot insert transaction metadata %d for %d updates", t.Id, len(t.Updates), err)
	}

	core.End("")
	return nil
}
