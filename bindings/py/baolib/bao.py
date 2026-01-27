import os
import json
import base64
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

from .baod import e8, j8, Data
from .baob import lib, consume


def set_bao_log_level(level: str):
    """Set the Bao log level (trace|debug|info|warn|error|fatal|panic)."""
    return consume(lib.bao_setLogLevel(e8(level)))


PrivateID = str
PublicID = str
Realm = str
KeyPair = Tuple[PublicID, PrivateID]


def newPrivateID() -> PrivateID:
    return consume(lib.bao_security_newPrivateID())

def publicID(private_id: PrivateID) -> PublicID:
    return consume(lib.bao_security_publicID(e8(private_id)))

def newKeyPair() -> KeyPair:
    res = consume(lib.bao_security_newKeyPair()) or {}
    pub = str(res.get("publicID", ""))
    priv = str(res.get("privateID", ""))
    return (pub, priv)

# Backward compatible alias for older callers
newKeyPairMust = newKeyPair

def key_pair_to_dict(pair: KeyPair) -> Dict[str, str]:
    return {"publicID": pair[0], "privateID": pair[1]}

def decodeID(id_str: str) -> Dict[str, Any]:
    '''
    Docstring for decodeID
    
    :param id_str: Description
    :type id_str: str
    :return: Description
    :rtype: Dict[str, Any]
    '''
    return consume(lib.bao_security_decodeID(e8(id_str)))

class Access:
    none = 0
    read = 1
    write = 2
    admin = 4
    read_write = read | write


@dataclass
class AccessChange:
    userId: PublicID
    access: int


@dataclass
class Message:
    subject: str
    body: str = ""
    attachments: List[str] = None
    fileInfo: Dict[str, Any] = None

    def __post_init__(self):
        self.attachments = self.attachments or []
        self.fileInfo = self.fileInfo or {}

    def toJson(self) -> str:
        return json.dumps(asdict(self))


class DB:
    def __init__(self, driver: str, path: str, ddl: str = "") -> "DB":
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        r = lib.bao_db_open(e8(driver), e8(path), e8(ddl))
        consume(r)
        self.hnd = r.hnd
        
    @staticmethod
    def default() -> "DB":
        home = os.path.expanduser("~")
        db_path = os.path.join(home, ".config", "bao.db")
        return DB("sqlite3", db_path)

    def close(self):
        if getattr(self, "hnd", 0):
            consume(lib.bao_db_close(self.hnd))
            self.hnd = 0

    def exec(self, query: str, args: Dict[str, Any]):
        r = lib.bao_db_exec(self.hnd, e8(query), j8(args))
        return consume(r)

    def fetch(self, query: str, args: Dict[str, Any], max_rows: int = 100000):
        r = lib.bao_db_fetch(self.hnd, e8(query), j8(args), max_rows)
        return consume(r)

    def fetch_one(self, query: str, args: Dict[str, Any]):
        r = lib.bao_db_fetch_one(self.hnd, e8(query), j8(args))
        return consume(r)

    def __del__(self):
        self.close()


class Store:
    def __init__(self, config: Dict[str, Any]):
        r = lib.bao_store_open(j8(config))
        consume(r)
        self.hnd = r.hnd
        self.config = config
        
    def close(self):
        if getattr(self, "hnd", 0):
            consume(lib.bao_store_close(self.hnd))
            self.hnd = 0

    def read_dir(self, path: str, filter: Optional[Dict[str, Any]] = None):
        payload = j8(filter or {})
        return consume(lib.bao_store_readDir(self.hnd, e8(path), payload))

    def stat(self, path: str):
        return consume(lib.bao_store_stat(self.hnd, e8(path)))

    def delete(self, path: str):
        return consume(lib.bao_store_delete(self.hnd, e8(path)))

    def __del__(self):
        self.close()


def _file_url(path: str) -> str:
    if path.startswith("file://"):
        return path
    p = Path(path).expanduser().absolute()
    return p.as_uri()

def local_store(id: str, path: str) -> Dict[str, Any]:
    base = _file_url(path)
    return {
        "id": id,
        "type": "local",
        "local": {"base": base},
    }

def s3_store(
    id: str,
    endpoint: str,
    bucket: str,
    access_key_id: str,
    secret_access_key: str,
    prefix: str = "",
    region: str = "",
    verbose: int = 0,
    proxy: str = "",
) -> Dict[str, Any]:
    return {
        "id": id,
        "type": "s3",
        "s3": {
            "endpoint": endpoint,
            "region": region,
            "bucket": bucket,
            "prefix": prefix,
            "auth": {
                "accessKeyId": access_key_id,
                "secretAccessKey": secret_access_key,
            },
            "verbose": verbose,
            "proxy": proxy,
        },
    }

def azure_store(
    id: str,
    account_name: str,
    account_key: str,
    share: str,
    base_path: str = "",
    verbose: int = 0,
) -> Dict[str, Any]:
    return {
        "id": id,
        "type": "azure",
        "azure": {
            "accountName": account_name,
            "accountKey": account_key,
            "share": share,
            "basePath": base_path,
            "verbose": verbose,
        },
    }

def webdav_store(
    id: str,
    host: str,
    username: str,
    password: str,
    base_path: str = "",
    port: int = 0,
    https: bool = False,
    verbose: int = 0,
) -> Dict[str, Any]:
    return {
        "id": id,
        "type": "webdav",
        "webdav": {
            "username": username,
            "password": password,
            "host": host,
            "port": port,
            "basePath": base_path,
            "verbose": verbose,
            "https": https,
        },
    }

def sftp_store(
    id: str,
    host: str,
    username: str,
    password: str = "",
    port: int = 0,
    key_file: str = "",
    base_path: str = "",
    verbose: int = 0,
) -> Dict[str, Any]:
    return {
        "id": id,
        "type": "sftp",
        "sftp": {
            "username": username,
            "password": password,
            "host": host,
            "port": port,
            "keyFile": key_file,
            "basePath": base_path,
            "verbose": verbose,
        },
    }

class File:
    def __init__(
        self,
        file_id: int = 0,
        name: str = "",
        size: int = 0,
        allocated_size: int = 0,
        mod_time: Optional[datetime] = None,
        is_dir: bool = False,
        flags: int = 0,
        attrs: bytes = b"",
        local_copy: str = "",
        key_id: int = 0,
        store_dir: str = "",
        store_name: str = "",
        author_id: PublicID = "",
    ):
        self.file_id = file_id
        self.name = name
        self.size = size
        self.allocated_size = allocated_size
        self.mod_time = mod_time
        self.is_dir = is_dir
        self.flags = flags
        self.attrs = attrs
        self.local_copy = local_copy
        self.key_id = key_id
        self.store_dir = store_dir
        self.store_name = store_name
        self.author_id = author_id

    @staticmethod
    def _parse_mod_time(value: Any) -> Optional[datetime]:
        if value is None:
            return None
        if isinstance(value, datetime):
            return value
        if isinstance(value, (int, float)):
            return datetime.fromtimestamp(value)
        if isinstance(value, str):
            try:
                return datetime.fromisoformat(value.replace("Z", "+00:00"))
            except ValueError:
                return None
        return None

    @staticmethod
    def _parse_attrs(value: Any) -> bytes:
        if value is None:
            return b""
        if isinstance(value, bytes):
            return value
        if isinstance(value, str):
            try:
                return base64.b64decode(value)
            except (ValueError, TypeError):
                return b""
        return b""

    @classmethod
    def from_dict(cls, payload: Optional[Dict[str, Any]]) -> "File":
        payload = payload or {}
        store_dir = payload.get("storeDir") or payload.get("store.ir") or ""
        store_name = payload.get("storeName") or payload.get("store.ame") or ""
        local_copy = payload.get("localCopy") or payload.get("local") or ""
        return cls(
            file_id=int(payload.get("id", 0) or 0),
            name=payload.get("name", "") or "",
            size=int(payload.get("size", 0) or 0),
            allocated_size=int(payload.get("allocatedSize", 0) or 0),
            mod_time=cls._parse_mod_time(payload.get("modTime")),
            is_dir=bool(payload.get("isDir", False)),
            flags=int(payload.get("flags", 0) or 0),
            attrs=cls._parse_attrs(payload.get("attrs")),
            local_copy=local_copy,
            key_id=int(payload.get("keyId", 0) or 0),
            store_dir=store_dir,
            store_name=store_name,
            author_id=payload.get("authorId", "") or "",
        )

    def __repr__(self) -> str:
        return (
            "File("
            f"id={self.file_id}, name={self.name!r}, "
            f"size={self.size}, allocated={self.allocated_size}, "
            f"mod_time={self.mod_time!r}, is_dir={self.is_dir}, "
            f"flags={self.flags}, key_id={self.key_id}, "
            f"author_id={self.author_id!r}"
            ")"
        )

class Vault:
    async_operation = 1 # write/read operations are async
    scheduled_operation = 2 # write/read operations are scheduled for a background time
    
    def __init__(self):
        self.hnd: int = 0
        self.id: str = ""
        self.userId: str = ""
        self.userPublicId: str = ""
        self.realm: str = ""
        self.store_config: Dict[str, Any] = {}
        self.author: str = ""
        self.config: Dict[str, Any] = {}

    @staticmethod
    def _from_result(r) -> "Vault":
        info = consume(r) or {}
        s = Vault()
        s.hnd = r.hnd
        s.id = info.get("id", "")
        s.userId = info.get("userId", "")
        s.userPublicId = info.get("userPublicId", "")
        s.realm = info.get("realm", "")
        s.store_config = info.get("storeConfig", {})
        s.author = info.get("author", "")
        s.config = info.get("config", {})
        return s

    @staticmethod
    def create(realm: Realm, identity: PrivateID, db: DB, store: Store, config: Dict[str, Any] = None) -> "Vault":
        config = config or {}
        r = lib.bao_vault_create(e8(realm), e8(identity), store.hnd, db.hnd, j8(config))
        return Vault._from_result(r)

    @staticmethod
    def open(realm: Realm, identity: PrivateID, db: DB, store: Store, config: Dict[str, Any], author: PublicID) -> "Vault":
        r = lib.bao_vault_open(e8(realm), e8(identity), db.hnd, store.hnd, j8(config), e8(author))
        return Vault._from_result(r)

    def close(self):
        if getattr(self, "hnd", 0):
            consume(lib.bao_vault_close(self.hnd))
            self.hnd = 0

    def sync_access(self, changes: List[AccessChange] = None, options: int = 0):
        '''
        Sync access changes from the remote store and optionaly apply new changes.
        
        :param self: Bao instance
        :param changes: List of access changes to apply
        :type changes: List[AccessChange]
        :param options: Options for syncing access
        :type options: int
        :return: Result of the sync operation
        :rtype: Any
        '''
        payload = [] if not changes else [asdict(c) for c in changes]
        return consume(lib.bao_vault_syncAccess(self.hnd, options, j8(payload)))

    def get_accesses(self) -> Dict[PublicID, int]:
        return consume(lib.bao_vault_getAccesses(self.hnd)) or {}

    def get_access(self, user: PublicID) -> int:
        return int(consume(lib.bao_vault_getAccess(self.hnd, e8(user))) or 0)

    def wait_files(self, file_ids: Optional[List[int]] = None):
        '''
        Wait for the specified files to be fully written/synced.
        
        :param self: Bao instance
        :param file_ids: List of file IDs to wait for
        :type file_ids: Optional[List[int]]
        :return: Result of the wait operation
        :rtype: Any
        '''
        payload = None if file_ids is None else j8(file_ids)
        return consume(lib.bao_vault_waitFiles(self.hnd, payload))

    def sync(self):
        return consume(lib.bao_vault_sync(self.hnd))

    def set_attribute(self, name: str, value: str, options: int = 0):
        return consume(lib.bao_vault_setAttribute(self.hnd, options, e8(name), e8(value)))

    def get_attribute(self, name: str, author: PublicID):
        return consume(lib.bao_vault_getAttribute(self.hnd, e8(name), e8(author)))

    def get_attributes(self, author: PublicID):
        return consume(lib.bao_vault_getAttributes(self.hnd, e8(author)))

    def read_dir(self, dir: str, since: Optional[datetime] = None, from_id: int = 0, limit: int = 0):
        since_sec = 0 if since is None else int(since.timestamp())
        payload = consume(lib.bao_vault_readDir(self.hnd, e8(dir), since_sec, from_id, limit))
        return [File.from_dict(item) for item in (payload or [])]

    def stat(self, name: str) -> File:
        return File.from_dict(consume(lib.bao_vault_stat(self.hnd, e8(name))))

    def read(self, name: str, dest: str, options: int = 0) -> File:
        payload = consume(lib.bao_vault_read(self.hnd, e8(name), e8(dest), options))
        return File.from_dict(payload)

    def write(self, dest: str, src: str = "", attrs: bytes = b"", options: int = 0) -> File:
        attrs = attrs or b""
        data = Data.from_byte_array(attrs)
        r = lib.bao_vault_write(self.hnd, e8(dest), e8(src), data, options)
        return File.from_dict(consume(r))

    def delete(self, name: str, options: int = 0):
        return consume(lib.bao_vault_delete(self.hnd, e8(name), options))

    def allocated_size(self) -> int:
        r = lib.bao_vault_allocatedSize(self.hnd)
        return consume(r)

    def __del__(self):
        self.close()

    def __repr__(self) -> str:
        return self.store_config.get("id", "") or self.id


class Rows:
    def __init__(self, hnd: int):
        self.hnd = hnd

    def next(self) -> bool:
        return bool(consume(lib.bao_replica_next(self.hnd)))

    def current(self):
        return consume(lib.bao_replica_current(self.hnd))

    def close(self):
        consume(lib.bao_replica_closeRows(self.hnd))
        self.hnd = 0


class Replica:
    def __init__(self, vault: Vault, db: DB) -> "Replica":
        r = lib.bao_replica_open(vault.hnd, db.hnd)
        consume(r)
        self.hnd = r.hnd

    def exec(self, query: str, args: Dict[str, Any]):
        return consume(lib.bao_replica_exec(self.hnd, e8(query), j8(args)))

    def query(self, query: str, args: Dict[str, Any]) -> Rows:
        r = lib.bao_replica_query(self.hnd, e8(query), j8(args))
        consume(r)
        return Rows(r.hnd)

    def fetch(self, query: str, args: Dict[str, Any], max_rows: int = 100000):
        return consume(lib.bao_replica_fetch(self.hnd, e8(query), j8(args), max_rows))

    def fetch_one(self, query: str, args: Dict[str, Any]):
        return consume(lib.bao_replica_fetchOne(self.hnd, e8(query), j8(args)))

    def sync(self) -> int:
        r = lib.bao_replica_sync(self.hnd)
        return consume(r)

    def cancel(self):
        return consume(lib.bao_replica_cancel(self.hnd))

class Mailbox:
    def __init__(self, vault: Vault, dir: str) -> "Mailbox":
        self.hnd = vault.hnd
        self.dir = dir
        
    def send(self, message: Message):
        return consume(lib.bao_mailbox_send(self.hnd, e8(self.dir), e8(message.toJson())))
    
    def receive(self, since: int = 0, from_id: int = 0) -> List[Message]:
        res = consume(lib.bao_mailbox_receive(self.hnd, e8(self.dir), since, from_id)) or []
        return [Message(**m) for m in res]
    
    def download(self, message: Dict[str, Any], attachment: int, dest: str):
        return consume(lib.bao_mailbox_download(self.hnd, e8(self.dir), j8(message), attachment, e8(dest)))  
