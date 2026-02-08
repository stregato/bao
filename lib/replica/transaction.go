package replica

import (
	"database/sql"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
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

// Sync writes the current transaction to the store.and reads all transactions that are not yet processed.
// It executes the updates in the transactions and commits them to the database.
// It runs two goroutines: one for writing the updates and one for reading and executing the transactions.
// It returns the number of updates processed or an error if something goes wrong.
func (ds *Replica) Sync() (int, error) {
	core.Start("")
	now := time.Now()

	ds.execLock.Lock()
	defer ds.execLock.Unlock()

	ls, err := ds.listUnreadTransactions()
	if err != nil {
		return 0, core.Error(core.DbError, "cannot get transactions files", err)
	}

	transactions, err := ds.readTransactionFiles(ls)
	if err != nil {
		return 0, core.Error(core.GenericError, "cannot read transactions", err)
	}

	if ds.transaction != nil {
		t, err := ds.writeUpdates()
		if err != nil {
			return 0, core.Error(core.DbError, "cannot write updates in sql layer", err)
		}
		transactions = append(transactions, t) // add the current transaction to the list
	}

	ds.queryLock.Lock()
	defer ds.queryLock.Unlock()

	updates, err := ds.processTransactions(transactions)
	if err != nil {
		return 0, core.Error(core.GenericError, "cannot process transactions", err)
	}

	core.End("elapsed %s", time.Since(now))
	return len(updates), nil
}

func (ds *Replica) writeUpdates() (transaction, error) {
	core.Start("writing updates for transaction with %d updates", len(ds.transaction.Updates))
	now := time.Now()
	t := *ds.transaction
	err := ds.transaction.tx.Rollback() // rollback the transaction to ensure it is not committed yet
	if err != nil {
		return transaction{}, core.Error(core.GenericError, "cannot rollback transaction", err)
	}

	attrs, err := msgpack.Marshal(t)
	if err != nil {
		return transaction{}, core.Error(core.DbError, "cannot marshal transaction %d in BaoQL.writeUpdates", ds.transaction.Id, err)
	}
	attrs, err = core.GzipCompress(attrs)
	if err != nil {
		return transaction{}, core.Error(core.DbError, "cannot compress transaction %d in BaoQL.writeUpdates", ds.transaction.Id, err)
	}
	name := strconv.FormatUint(core.SnowID(), 16) // generate a unique name for the transaction file
	file, err := ds.vault.Write(path.Join(sqlLayerDir, name), "", attrs, 0, nil)
	if err != nil {
		return transaction{}, core.Error(core.DbError, "cannot write transaction %d in BaoQL.writeUpdates", ds.transaction.Id, err)
	}

	ds.transaction = nil // reset the transaction to nil after writing
	t.Id = file.Id       // set the transaction Id from the file Id
	core.End("name %s, elapsed %s", name, core.Since(now))
	return t, nil
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

func (ds *Replica) listUnreadTransactions() (ls []vault.File, err error) {
	core.Start("")
	files, err := ds.vault.ReadDir(sqlLayerDir, time.Time{}, ds.lastId, 0)
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
		transaction.Id = max(fi.Id, transaction.Id) // set the transaction Id from the file Id
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
	}
	core.End("%d updates", len(updates))
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
	for _, u := range t.Updates {
		_, err := ds.db.Exec(u.Key, u.Args)
		if err != nil {
			return core.Error(core.GenericError, "cannot execute transaction %s", u.Key, err)
		}
	}
	ds.lastId = int64(t.Id)

	_, err := ds.vault.DB.Exec("INSERT_TRANSACTION_METADATA", sqlx.Args{"vault": ds.vault.ID, "id": t.Id,
		"tm": t.Tm, "success": true})
	if err != nil {
		return core.Error(core.DbError, "cannot insert transaction metadata %d for %d updates", t.Id, len(t.Updates), err)
	}

	core.End("")
	return nil
}
