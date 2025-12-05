-- INIT
CREATE TABLE IF NOT EXISTS mio_configs (
    node    VARCHAR(128) NOT NULL, 
    k       VARCHAR(64) NOT NULL, 
    s       VARCHAR(64) NOT NULL,
    i       INTEGER NOT NULL,
    b       BLOB,
    CONSTRAINT pk_safe_key PRIMARY KEY(node,k)
);

-- STASH_GET_CONFIG
SELECT s, i, b FROM mio_configs WHERE node=:node AND k=:key

-- STASH_SET_CONFIG
INSERT INTO mio_configs(node,k,s,i,b) VALUES(:node,:key,:s,:i,:b)
	ON CONFLICT(node,k) DO UPDATE SET s=:s,i=:i,b=:b
	WHERE node=:node AND k=:key

-- STASH_DEL_CONFIG
DELETE FROM mio_configs WHERE node=:node

-- STASH_LIST_CONFIG
SELECT k FROM mio_configs WHERE node=:node

-- INIT
CREATE TABLE IF NOT EXISTS mio_files (
    safeID          VARCHAR(256)    NOT NULL,
    name            VARCHAR(256)    NOT NULL,
    dir             VARCHAR(4096)   NOT NULL,
    id              INTEGER         NOT NULL,
    creator         VARCHAR(256)    NOT NULL,
    groupName       VARCHAR(256)    NOT NULL,
    tags            VARCHAR(4096)   NOT NULL,
    encryptionKey   VARCHAR(256)    NOT NULL,
    modTime         INTEGER         NOT NULL,
    size            INTEGER         NOT NULL,
    localCopy       VARCHAR(4096)   NOT NULL,
    copyTime    INTEGER         NOT NULL,
    attributes      BLOB,
    PRIMARY KEY(safeID, name, dir, id)
);

-- INIT
CREATE INDEX IF NOT EXISTS idx_mio_files_id ON mio_files(id)

-- INIT
CREATE INDEX IF NOT EXISTS idx_mio_files_groupName ON mio_files(groupName)

-- INIT
CREATE INDEX IF NOT EXISTS idx_mio_files_tags ON mio_files(tags)

-- INIT
CREATE INDEX IF NOT EXISTS idx_mio_files_modTime ON mio_files(modTime)

-- INIT
CREATE INDEX IF NOT EXISTS idx_mio_files_name ON mio_files(name)

-- STASH_STORE_FILE
INSERT INTO mio_files(safeID,name,dir,id,creator,groupName,tags,encryptionKey,modTime,size,localCopy, 
    copyTime, attributes) VALUES(:safeID,:name,:dir,:id,:creator,:groupName,:tags,:encryptionKey,
    :modTime,:size,:localCopy,:copyTime,:attributes) ON CONFLICT(safeID,name,dir,id) DO UPDATE
    SET creator=:creator,groupName=:groupName,tags=:tags,encryptionKey=:encryptionKey,modTime=:modTime,
    size=:size,localCopy=:localCopy,copyTime=:copyTime,attributes=:attributes
    WHERE id=:id AND safeID=:safeID AND name=:name AND dir=:dir

-- STASH_STORE_DIR
INSERT INTO mio_files(safeID,name,dir,id,creator,groupName,tags,encryptionKey,modTime,size,localCopy,copyTime)
    VALUES(:safeID,:name,:dir,0,'','','','',0,0,'',0)
    ON CONFLICT(safeID,name,dir,id) DO NOTHING

-- STASH_UPDATE_LOCALCOPY
UPDATE mio_files SET localCopy=:localCopy, copyTime=:copyTime 
    WHERE safeID=:safeID AND name=:name AND dir=:dir AND id=:id

-- STASH_GET_LAST_ID
SELECT id FROM mio_files WHERE dir=:dir ORDER BY id DESC LIMIT 1

-- STASH_GET_FILES_BY_DIR
SELECT id,name,dir,groupName,tags,modTime,size,creator,attributes,localCopy,copyTime,encryptionKey 
    FROM mio_files WHERE dir=:dir AND safeID=:safeID
    AND (:name = '' OR name = :name)
    AND (:groupName = '' OR groupName = :groupName)
    AND (:tag = '' OR tags LIKE '% ' || :tag || ' %')
    AND (:creator = '' OR creator = :creator)
    AND (:before < 0 OR modTime < :before)
    AND (:after < 0 OR modTime > :after)
    AND (:prefix = '' OR name LIKE :prefix || '%')
    AND (:suffix = '' OR name LIKE '%' || :suffix)
    #orderBy
    LIMIT CASE WHEN :limit = 0 THEN -1 ELSE :limit END OFFSET :offset

-- STASH_GET_FILE_BY_NAME
SELECT  id,groupName,tags,modTime,size,creator,attributes,localCopy,copyTime,encryptionKey  
    FROM mio_files WHERE safeID=:safeID AND dir=:dir AND name=:name ORDER BY id DESC LIMIT 1

-- STASH_GET_GROUP_NAME 
SELECT DISTINCT groupName FROM mio_files WHERE safeID=:safeID AND dir = :dir AND name = :name 

-- STASH_DELETE_FILE
DELETE FROM mio_files WHERE safeID=:safeID AND id=:id

-- STASH_DELETE_DIR
DELETE FROM mio_files WHERE safeID=:safeID AND dir=:dir AND id=0 AND NOT EXISTS(SELECT 1 FROM mio_files WHERE safeID=:safeID AND dir=:dir AND id>0)

-- STASH_REnameFILE
UPDATE mio_files SET name=:newName, dir=:newDir WHERE safeID=:safeID AND id=:id AND name=:oldName AND dir=:oldDir

-- INIT
CREATE TABLE IF NOT EXISTS mio_file_async (
    safeID      VARCHAR(256)    NOT NULL,
    id          VARCHAR(256)    NOT NULL,
    deleteSrc   INTEGER         NOT NULL,
    localCopy   VARCHAR(4096)   NOT NULL,
    operation   VARCHAR(0)      NOT NULL,
    file        BLOB NOT        NULL,
    data        BLOB,
    CONSTRAINT pk_mio_file_async PRIMARY KEY(safeID,id)
);

-- STASH_INSERT_FILE_ASYNC
INSERT INTO mio_file_async(safeID,id,deleteSrc,localCopy,operation,file,data) 
    VALUES(:safeID,:id,:deleteSrc,:localCopy,:operation,:file,:data)

-- STASH_GET_FILE_ASYNC
SELECT file,data, deleteSrc, localCopy, operation FROM mio_file_async WHERE safeID=:safeID AND id=:id

-- STASH_GET_FILES_ASYNC
SELECT id,file,data, deleteSrc, localCopy, operation FROM mio_file_async WHERE safeID=:safeID

-- STASH_DEL_FILE_ASYNC
DELETE FROM mio_file_async WHERE safeID=:safeID AND id=:id

-- INIT
CREATE TABLE IF NOT EXISTS mio_tx (
    safeID      TEXT, 
    groupName   TEXT, 
    kind        TEXT,
    id      TEXT,
    CONSTRAINT pk_mio_tx_key PRIMARY KEY(safeID,groupName,kind)
)

-- STASH_STORE_TX
INSERT INTO mio_tx (safeID,groupName,kind,id) VALUES(:safeID,:groupName,:kind,:id)
    ON CONFLICT(safeID,groupName,kind) DO UPDATE SET id=:id
    WHERE safeID=:safeID AND groupName=:groupName AND kind=:kind

-- STASH_GET_TX
SELECT kind, id FROM mio_tx WHERE safeID = :safeID AND groupName = :groupName

-- STASH_DEL_TX_KIND
DELETE FROM mio_tx WHERE safeID = :safeID AND groupName = :groupName AND kind = :kind
