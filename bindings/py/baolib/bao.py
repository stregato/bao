import os
import json
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

from .baod import e8, j8, Data
from .baob import lib, consume


def set_bao_log_level(level: str):
    """Set the Bao log level (trace|debug|info|warn|error|fatal|panic)."""
    return consume(lib.bao_setLogLevel(e8(level)))


PrivateID = str
PublicID = str


def newPrivateID() -> PrivateID:
    return consume(lib.bao_newPrivateID())


def publicID(private_id: PrivateID) -> PublicID:
    return consume(lib.bao_publicID(e8(private_id)))


class Access:
    read = 1
    write = 2
    admin = 4
    read_write = read | write


class Groups:
    users = "users"
    admins = "admins"
    public = "public"
    sql = "sql"
    blockchain = "#blockchain"
    cleanup = "#cleanup"


@dataclass
class AccessChange:
    group: str
    access: int
    userId: PublicID


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
    def __init__(self, handle: int):
        self.hnd = handle

    @staticmethod
    def open(driver: str, path: str, ddl: str = "") -> "DB":
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        r = lib.bao_openDB(e8(driver), e8(path), e8(ddl))
        consume(r)
        return DB(r.hnd)

    @staticmethod
    def default() -> "DB":
        home = os.path.expanduser("~")
        db_path = os.path.join(home, ".config", "bao.db")
        return DB.open("sqlite3", db_path)

    def close(self):
        if getattr(self, "hnd", 0):
            consume(lib.bao_closeDB(self.hnd))
            self.hnd = 0

    def exec(self, query: str, args: Dict[str, Any]):
        r = lib.bao_dbExec(self.hnd, e8(query), j8(args))
        return consume(r)

    def fetch(self, query: str, args: Dict[str, Any], max_rows: int = 100000):
        r = lib.bao_dbFetch(self.hnd, e8(query), j8(args), max_rows)
        return consume(r)

    def fetch_one(self, query: str, args: Dict[str, Any]):
        r = lib.bao_dbFetchOne(self.hnd, e8(query), j8(args))
        return consume(r)

    def __del__(self):
        self.close()


class Bao:
    def __init__(self):
        self.hnd: int = 0
        self.id: str = ""
        self.userId: str = ""
        self.userPublicId: str = ""
        self.url: str = ""
        self.author: str = ""
        self.config: Dict[str, Any] = {}

    @staticmethod
    def _from_result(r) -> "Bao":
        info = consume(r) or {}
        s = Bao()
        s.hnd = r.hnd
        s.id = info.get("id", "")
        s.userId = info.get("userId", "")
        s.userPublicId = info.get("userPublicId", "")
        s.url = info.get("url", "")
        s.author = info.get("author", "")
        s.config = info.get("config", {})
        return s

    @staticmethod
    def create(db: DB, identity: PrivateID, url: str, settings: Dict[str, Any] = None) -> "Bao":
        settings = settings or {}
        r = lib.bao_create(db.hnd, e8(identity), e8(url), j8(settings))
        return Bao._from_result(r)

    @staticmethod
    def open(db: DB, identity: PrivateID, url: str, author: PublicID) -> "Bao":
        r = lib.bao_open(db.hnd, e8(identity), e8(url), e8(author))
        return Bao._from_result(r)

    def close(self):
        if getattr(self, "hnd", 0):
            consume(lib.bao_close(self.hnd))
            self.hnd = 0

    def sync_access(self, changes: List[AccessChange] = None, options: int = 0):
        payload = [] if not changes else [asdict(c) for c in changes]
        return consume(lib.bao_syncAccess(self.hnd, options, j8(payload)))

    def get_access(self, group: str):
        return consume(lib.bao_getAccess(self.hnd, e8(group)))

    def get_groups(self, user: PublicID):
        return consume(lib.bao_getGroups(self.hnd, e8(user)))

    def wait_files(self, file_ids: Optional[List[int]] = None):
        payload = None if file_ids is None else j8(file_ids)
        return consume(lib.bao_waitFiles(self.hnd, payload))

    def list_groups(self):
        return consume(lib.bao_listGroups(self.hnd))

    def sync(self, groups: Optional[List[str]] = None):
        payload = None if groups is None else j8(groups)
        return consume(lib.bao_sync(self.hnd, payload))

    def set_attribute(self, name: str, value: str, options: int = 0):
        return consume(lib.bao_setAttribute(self.hnd, options, e8(name), e8(value)))

    def get_attribute(self, name: str, author: PublicID):
        return consume(lib.bao_getAttribute(self.hnd, e8(name), e8(author)))

    def get_attributes(self, author: PublicID):
        return consume(lib.bao_getAttributes(self.hnd, e8(author)))

    def read_dir(self, dir: str, since: Optional[datetime] = None, from_id: int = 0, limit: int = 0):
        since_sec = 0 if since is None else int(since.timestamp())
        return consume(lib.bao_readDir(self.hnd, e8(dir), since_sec, from_id, limit))

    def stat(self, name: str):
        return consume(lib.bao_stat(self.hnd, e8(name)))

    def read(self, name: str, dest: str, options: int = 0):
        return consume(lib.bao_read(self.hnd, e8(name), e8(dest), options))

    def write(self, dest: str, group: str, src: str = "", attrs: bytes = b"", options: int = 0):
        attrs = attrs or b""
        data = Data.from_byte_array(attrs)
        r = lib.bao_write(self.hnd, e8(dest), e8(src), e8(group), data, options)
        return consume(r)

    def delete(self, name: str, options: int = 0):
        return consume(lib.bao_delete(self.hnd, e8(name), options))

    def allocated_size(self) -> int:
        r = lib.bao_allocatedSize(self.hnd)
        return consume(r)

    def baoql(self, group: str, db: DB) -> "SqlLayer":
        r = lib.baoql_layer(self.hnd, e8(group), db.hnd)
        consume(r)
        return SqlLayer(r.hnd)

    def send(self, dir: str, group: str, message: Message):
        return consume(lib.mailbox_send(self.hnd, e8(dir), e8(group), e8(message.toJson())))

    def receive(self, dir: str, since: int = 0, from_id: int = 0) -> List[Message]:
        res = consume(lib.mailbox_receive(self.hnd, e8(dir), since, from_id)) or []
        return [Message(**m) for m in res]

    def download(self, dir: str, message: Dict[str, Any], attachment: int, dest: str):
        return consume(lib.mailbox_download(self.hnd, e8(dir), j8(message), attachment, e8(dest)))

    def __del__(self):
        self.close()

    def __repr__(self) -> str:
        return self.url or self.id


class Rows:
    def __init__(self, hnd: int):
        self.hnd = hnd

    def next(self) -> bool:
        return bool(consume(lib.baoql_next(self.hnd)))

    def current(self):
        return consume(lib.baoql_current(self.hnd))

    def close(self):
        consume(lib.baoql_closeRows(self.hnd))
        self.hnd = 0


class SqlLayer:
    def __init__(self, hnd: int):
        self.hnd = hnd

    def exec(self, query: str, args: Dict[str, Any]):
        return consume(lib.baoql_exec(self.hnd, e8(query), j8(args)))

    def query(self, query: str, args: Dict[str, Any]) -> Rows:
        r = lib.baoql_query(self.hnd, e8(query), j8(args))
        consume(r)
        return Rows(r.hnd)

    def fetch(self, query: str, args: Dict[str, Any], max_rows: int = 100000):
        return consume(lib.baoql_fetch(self.hnd, e8(query), j8(args), max_rows))

    def fetch_one(self, query: str, args: Dict[str, Any]):
        return consume(lib.baoql_fetchOne(self.hnd, e8(query), j8(args)))

    def sync_tables(self) -> int:
        r = lib.baoql_sync_tables(self.hnd)
        return consume(r)

    def cancel(self):
        return consume(lib.baoql_cancel(self.hnd))
