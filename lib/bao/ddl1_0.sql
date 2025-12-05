-- INIT 1.0
CREATE TABLE IF NOT EXISTS staged_changes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    store VARCHAR(1024) NOT NULL,
    changeType INTEGER NOT NULL,
    change BLOB NOT NULL
);

-- INSERT_STAGED_CHANGE 1.0
INSERT INTO staged_changes (store, changeType, change) VALUES (:store, :changeType, :change)

-- DELETE_STAGED_CHANGES 1.0
DELETE FROM staged_changes WHERE store = :store

-- GET_STAGED_CHANGES 1.0
SELECT changeType, change FROM staged_changes WHERE store = :store ORDER BY id ASC

-- INIT 1.0
CREATE TABLE IF NOT EXISTS blocks (
    store VARCHAR(1024) NOT NULL,
    name VARCHAR(256) NOT NULL,
    showId INTEGER NOT NULL,
    hash BLOB NOT NULL,
    payload BLOB NOT NULL,
    PRIMARY KEY(store, name)
);

-- INIT 1.0
CREATE INDEX IF NOT EXISTS idx_blocks_store_showId ON blocks (store, showId);

-- INIT 1.0
CREATE INDEX IF NOT EXISTS idx_blocks_hash ON blocks (hash);

-- SET_BLOCK 1.0
INSERT INTO blocks(store, name, showId, hash, payload) VALUES (:store, :name, :showId, :hash, :payload)

-- GET_BLOCKS 1.0
SELECT name, hash, payload FROM blocks WHERE store=:store ORDER BY showId ASC

-- GET_LAST_HASH 1.0
SELECT hash FROM blocks WHERE store=:store ORDER BY showId DESC LIMIT 1

-- GET_BLOCKS_BY_HASH 1.0
SELECT name, showId, payload FROM blocks WHERE store=:store AND hash=:hash

-- INIT 1.0
CREATE TABLE IF NOT EXISTS keys (
    id INTEGER NOT NULL,
    store VARCHAR(1024) NOT NULL,
    "group" VARCHAR(256) NOT NULL,
    key BLOB,
    tm INTEGER NOT NULL,
    PRIMARY KEY(store, id)
);

-- SET_KEY 1.0
INSERT OR REPLACE INTO keys (id, store, "group", key, tm) VALUES (:id, :store, :group, :key, :tm)    

-- GET_KEY 1.0
SELECT key FROM keys WHERE id=:id

-- GET_SCOPE 1.0
SELECT "group" FROM keys WHERE id=:id

-- GET_LAST_KEY 1.0
SELECT id, key FROM keys WHERE store=:store AND "group"=:group ORDER BY id DESC LIMIT 1

-- GET_KEYS 1.0
SELECT id, key FROM keys WHERE store=:store AND "group"=:group

-- GET_KEYS_FOR_SCOPE 1.0
SELECT id, key FROM keys WHERE store=:store AND "group"=:group

-- GET_ALL_KEYS 1.0
SELECT id, key FROM keys WHERE store=:store

-- GET_SCOPES 1.0
SELECT DISTINCT "group" FROM keys WHERE store=:store

-- INIT 1.0
CREATE TABLE IF NOT EXISTS ids (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    publicId CHAR(87) NOT NULL,
    UNIQUE(publicId)
);

-- SET_ID 1.0
INSERT OR IGNORE INTO ids (publicId) VALUES (:publicId);

-- GET_ID 1.0
SELECT id FROM ids WHERE publicId = :publicId;

-- INIT 1.0
CREATE TABLE IF NOT EXISTS users (
    store VARCHAR(1024) NOT NULL,
    "group" VARCHAR(256) NOT NULL,
    id INTEGER NOT NULL,
    access INTEGER NOT NULL,
    PRIMARY KEY(store, "group", id)
);

-- SET_USER 1.0
INSERT OR REPLACE INTO users (store, "group", id, access)
VALUES (:store, :group, :id, :access);

-- REMOVE_USER 1.0
DELETE FROM users WHERE store=:store AND "group"=:group AND id=(SELECT id FROM ids WHERE publicId=:publicId)

-- REMOVE_USERS 1.0
DELETE FROM users WHERE store=:store AND "group"=:group

-- GET_USERS 1.0
SELECT ids.publicId, users.access FROM users users INNER JOIN ids ids ON users.id = ids.id
WHERE users.store = :store AND users."group" = :group

-- GET_GROUPS 1.0
SELECT DISTINCT users."group", users.access FROM users users INNER JOIN ids ids ON users.id = ids.id
WHERE users.store = :store AND ids.publicId = :publicId

-- SET_GROUPS 1.0
INSERT OR REPLACE INTO users (store, "group", id, access)
VALUES (:store, :group, :id, :access);

-- INIT 1.0
CREATE TABLE IF NOT EXISTS settings (
    id VARCHAR(2048) NOT NULL,
    valueAsString VARCHAR(1024),
    valueAsInt INTEGER,
    valueAsReal REAL,
    valueAsBlob BLOB,
    PRIMARY KEY(id)
);

-- SET_SETTING 1.0
INSERT OR REPLACE INTO settings (id, valueAsString, valueAsInt, valueAsReal, valueAsBlob)
VALUES (:id, :valueAsString, :valueAsInt, :valueAsReal, :valueAsBlob)

-- GET_SETTING 1.0
SELECT valueAsString, valueAsInt, valueAsReal, valueAsBlob FROM settings WHERE id=:id

-- INIT 1.1
CREATE TABLE IF NOT EXISTS attributes (
    store VARCHAR(1024) NOT NULL,
    name VARCHAR(256) NOT NULL,
    value VARCHAR(4096) NOT NULL,
    id INTEGER,
    PRIMARY KEY(store, name, id)
);

-- SET_ATTRIBUTE 1.1
INSERT OR REPLACE INTO attributes (store, name, value, id) VALUES (:store, :name, :value, :id);

-- GET_ATTRIBUTE 1.1
SELECT value FROM attributes WHERE store=:store AND name=:name AND (id IS :id OR (id IS NULL AND :id IS NULL));

-- GET_ATTRIBUTES 1.1
SELECT name, value FROM attributes WHERE store=:store AND (id IS :id OR (id IS NULL AND :id IS NULL));

-- INIT 1.0
CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    store VARCHAR(1024) NOT NULL,
    storeDir VARCHAR(32) NOT NULL,
    storeName VARCHAR(32) NOT NULL,
    dir VARCHAR(4096) NOT NULL,
    name VARCHAR(256) NOT NULL,
    "group" VARCHAR(64),
    localCopy VARCHAR(4096) DEFAULT "",
    modTime INTEGER NOT NULL,
    size INTEGER NOT NULL,
    allocatedSize INTEGER NOT NULL,
    flags INTEGER NOT NULL,
    authorId VARCHAR(100) NOT NULL,
    keyId INTEGER NOT NULL,
    attrs BLOB
);

-- INIT 1.0
CREATE INDEX IF NOT EXISTS idx_files_store_dir ON files (store, dir);
CREATE INDEX IF NOT EXISTS idx_files_store_storeDir ON files (store, storeDir);
CREATE INDEX IF NOT EXISTS idx_files_store_storeDir_storeName ON files (store, storeDir, storeName);
CREATE INDEX IF NOT EXISTS idx_files_store_dir_name ON files (store, dir, name);
CREATE INDEX IF NOT EXISTS idx_files_store_dir_name_modTime ON files (store, dir, name, modTime);
CREATE INDEX IF NOT EXISTS idx_files_name_modTime ON files(name, modTime);

-- GET_LAST_STORE_DIR 1.0
SELECT storeDir FROM files WHERE store = :store AND storeDir LIKE :baseDir || '%'
ORDER BY id DESC LIMIT 1

-- GET_STORE_NAMES_IN_STORE_DIR 1.0
SELECT storeName FROM files WHERE store=:store AND storeDir=:storeDir ORDER BY modTime

-- SET_FILE 1.0
INSERT INTO files (store, storeDir, storeName, dir, name, "group", localCopy, modTime, size, allocatedSize, flags, authorId, keyId, attrs)
VALUES (:store, :storeDir, :storeName, :dir, :name, :group, :localCopy, :modTime, :size, :allocatedSize, :flags, :authorId, :keyId, :attrs)

-- SET_FLAGS_IN_FILE 1.0
UPDATE files SET flags = :flagsM WHERE ID = :id

-- SET_DIR 1.0
INSERT INTO files (store, storeDir, storeName, dir, name, "group", modTime, size, allocatedSize, flags, authorId, keyId, attrs)
SELECT :store, "", "", :dir, :name, :group, 0, 0, 0, 0, 0, 0, NULL
WHERE NOT EXISTS (
    SELECT 1 FROM files WHERE store = :store AND dir = :dir AND name = :name AND storeDir = :storeDir AND storeName = :storeName AND modTime = 0
);

-- GET_FILES_WITH_FLAGS 1.0
SELECT sf.id, sf.name, sf."group", sf.modTime, sf.size, sf.allocatedSize, sf.flags, sf.attrs
FROM files sf WHERE sf.store = :store AND sf.flags & :flagsM != 0 ORDER BY sf.id ASC;

-- GET_FILES_IN_DIR 1.0
SELECT sf.id, sf.name, sf."group", sf.localCopy, sf.modTime, sf.size, sf.allocatedSize, sf.flags, sf.attrs, sf.authorId, sf.keyId, sf.storeDir, sf.storeName
FROM files sf
JOIN (
    SELECT name, MAX(modTime) AS maxModTime
    FROM files
    WHERE store = :store AND dir = :dir AND modTime >= :since AND id > :afterId
    GROUP BY name
) latest ON sf.name = latest.name AND sf.modTime = latest.maxModTime
WHERE sf.store = :store AND sf.dir = :dir AND sf.modTime >= :since and sf.id > :afterId
LIMIT :limit;

-- GET_FILE_BY_ID 1.0
SELECT id, storeDir, storeName, dir, name, "group", localCopy, modTime, size, allocatedSize, flags, authorId, keyId, attrs FROM files WHERE store = :store AND id = :id

-- GET_FILE_BY_NAME 1.0
SELECT id, dir, name, "group", storeDir, storeName, localCopy, modTime, size, allocatedSize, flags, authorId, keyId, attrs FROM files 
WHERE store = :store AND dir = :dir AND 
name = :name ORDER BY modTime LIMIT 1 OFFSET :version

-- GET_FILE_VERSIONS 1.0
SELECT id, modTime, size, allocatedSize, flags FROM files WHERE store=:store AND dir=:dir AND 
name=:name ORDER BY modTime DESC 

-- GET_LATEST_STORE_DIR 1.0
SELECT storeDir FROM files WHERE store = :store AND storeDir LIKE :baseDir || '%'
ORDER BY id DESC LIMIT 1

-- STAT_FILE 1.0
SELECT id, modTime, size, allocatedSize, flags, attrs, "group" FROM files 
WHERE store = :store AND dir = :dir AND name = :name 
ORDER BY modTime DESC LIMIT 1

-- GET_LAST_FILE 1.0
SELECT storeDir, storeName, modTime, size, allocatedSize, flags, "group" FROM files 
WHERE store = :store AND dir = :dir AND name = :name
ORDER BY modTime DESC LIMIT 1 OFFSET :version

-- GET_STORE_NAMES 1.0
SELECT storeName FROM files 
WHERE store = :store AND storeDir = :storeDir

-- UPDATE_FILE_ALLOCATED_SIZE 1.0
UPDATE files SET allocatedSize = :allocatedSize WHERE store = :store AND id = :id

-- UPDATE_FILE_FLAGS 1.0
UPDATE files SET flags = :flags WHERE store = :store AND id = :id;

-- UPDATE_FILE_LOCAL_NAME 1.0
UPDATE files SET localCopy = :localCopy WHERE store = :store AND id = :id;

-- GET_FILE_IDS_BY_FLAGS 1.0
SELECT id FROM files WHERE store = :store AND (flags & :flagsMask) != 0 ORDER BY id ASC;

-- DELETE_FILES_BEFORE_MODTIME 1.0
DELETE FROM files WHERE store = :store AND modTime > 0 AND modTime < :modTime;

-- CALCULATE_ALLOCATED_SIZE 1.0
SELECT COALESCE(SUM(allocatedSize), 0) FROM files WHERE store = :store;

-- INIT 1.0
CREATE TABLE IF NOT EXISTS transaction_metadata (
    store VARCHAR(1024) NOT NULL,
    "group" VARCHAR(256) NOT NULL,
    id INTEGER NOT NULL,
    tm INTEGER NOT NULL,
    success INTEGER NOT NULL,
    PRIMARY KEY(store, "group", id)
);

-- INSERT_TRANSACTION_METADATA 1.0
INSERT INTO transaction_metadata (store, "group", id, tm, success) VALUES (:store, :group, :id, :tm, :success);

-- GET_LAST_TRANSACTIONS_METADATA 1.0
SELECT id, tm FROM transaction_metadata WHERE store=:store AND "group"=:group ORDER BY id DESC LIMIT :limit;

-- GET_LAST_TRANSACTION_METADATA_ID 1.0
SELECT id FROM transaction_metadata WHERE store=:store AND "group"=:group ORDER BY id DESC LIMIT 1;

-- DELETE_TRANSACTION_METADATA 1.0
DELETE FROM transaction_metadata WHERE store=:store AND "group"=:group AND tm < :tm;
