import json
import os
import tempfile
import appdirs
from dataclasses import asdict, dataclass
from typing import List, NewType, Union
from datetime import datetime
import base64
from urllib.parse import urlparse
from functools import lru_cache

from .baod import e8, j8, o8, Data
from .baob import lib, consume


def set_bao_log_level(level: str):
    r = lib.bao_setLogLevel(e8(level))
    return consume(r)


PrivateID = NewType('PrivateID', str)
PublicID = NewType('PublicID', str)

def newPrivateID() -> PrivateID: 
    r = consume(lib.bao_newPrivateID())
    return r

def publicID(privateId: PrivateID) -> PublicID:
    r = consume(lib.bao_publicID(e8(privateId)))
    return r
    

Access = NewType('Access', int)
Access.read = Access(1)
Access.write = Access(2)
Access.admin = Access(4)
    
class DB():
    def __init__(self, filename: str):
        r = lib.bao_openDB(e8(filename))
        self.hnd = r.hnd
    
    def __del__(self):
        if self.hnd:
            r = lib.bao_closeDB(self.hnd)
            return consume(r)    

class Groups:
    admin = "admin"
    public = "public"
    user = "user"


@dataclass
class Message:
    subject: str
    body: str
    attachments: List[str]
    fileInfo: dict
    
    def __init__(self, subject: str, body: str, attachments: List[str] = [], fileInfo: dict = {}):
        self.subject = subject
        self.body = body
        self.attachments = attachments
        self.fileInfo = fileInfo
    
    def toJson(self):
        return json.dumps(asdict(self))
    
    @staticmethod
    def fromJson(jsonStr: str):
        messages = json.loads(jsonStr)
        return messages.map(lambda m: Message(m["subject"], m["body"], m["attachments"]))

class Bao():
    @staticmethod
    def create(db: DB, creator: PrivateID, url: str, options: int = 0):
        """
        Create a new Bao vault with the specified database, creator, URL, and configuration.
        """
        r = lib.bao_create(db.hnd, e8(creator), e8(url), options)
        s = Bao()
        for k, v in consume(r).items():
            setattr(s, k, v)
        s.hnd = r.hnd
        s.db = db
        return s

    @staticmethod
    def open(db: DB, identity: PrivateID, author: PublicID, url: str, options: int = 0):
        """
        Open an existing Bao vault with the specified database, identity, author, and URL.
        """
        r = lib.bao_open(db.hnd, e8(identity), e8(author), e8(url), options)
        s = Bao()
        for k, v in consume(r).items():
            setattr(s, k, v)
        s.hnd = r.hnd
        return s

    def __del__(self):
        """
        Close the Bao handle.
        """
        if getattr(self, "hnd", None):
            r = lib.bao_close(self.hnd)
            return consume(r)
    
    def set_retention(self, tm: int, size: int):
        """
        Set the retention time (seconds) and size (bytes) for the Bao vault.
        """
        r = lib.bao_setRetention(self.hnd, tm, size)
        return consume(r)

    def set_access(self, groupName: str, access: dict[PublicID, Access]):
        """
        Set the access rights for the specified group in the Bao vault. Access is the dictionary mapping a
        publid ID to a permission mask
        """
        r = lib.bao_setAccess(self.hnd, e8(groupName), j8(access))
        return consume(r)

    def get_access(self, groupName: str):
        """
        Get the access rights for the specified group in the Bao vault.
        """
        r = lib.bao_getAccess(self.hnd, e8(groupName))
        return consume(r)

    def read_dir(self, dir: str, after: int, fromId: int, limit: int):
        """
        Read the specified directory from the Bao vault.
        """
        r = lib.bao_readDir(self.hnd, e8(dir), after, fromId, limit)
        return consume(r)

    def get_group(self, name: str):
        """
        Get the group name of the specified file.
        """
        r = lib.bao_getGroup(self.hnd, e8(name))
        return consume(r)

    def get_author(self, name: str):
        """
        Get the author of the specified file.
        """
        r = lib.bao_getAuthor(self.hnd, e8(name))
        return consume(r)

    def read_file(self, name: str, dest: str, options: int):
        """
        Read the specified file from the Bao vault.
        """
        r = lib.bao_readFile(self.hnd, e8(name), e8(dest), options)
        return consume(r)

    def write_file(self, name: str, group: str, src: str, options: int = 0):
        """
        Write the specified file to the Bao vault.
        """
        r = lib.bao_writeFile(self.hnd, e8(name), e8(group), e8(src), options)
        return consume(r)

    def read_data(self, name: str, options: int = 0):
        """
        Read the specified data from the Bao vault.
        """
        r = lib.bao_readData(self.hnd, e8(name), options)
        return consume(r, returnBytes=True)

    def write_data(self, name: str, group: str, data: bytes, options: int = 0):
        """
        Write the specified data to the Bao vault.
        """
        d = Data.from_byte_array(data)
        r = lib.bao_writeData(self.hnd, e8(name), e8(group), d, options)
        return consume(r)

    def sqlLayer(self, dir: str, group: str, ddls: dict):
        """
        Execute SQL commands on the Bao vault.
        """
        r = lib.bao_sqlLayer(self.hnd, e8(dir), e8(group), j8(ddls))
        consume(r)
        return SqlLayer(r.hnd)

    def send(self, dir: str, group: str, message: Message):
        """
        Send a message to the specified group in the Bao vault.
        """
        m = message.toJson()
        r = lib.bao_send(self.hnd, e8(dir), e8(group), e8(m))
        return consume(r)

    def receive(self, dir: str, since: int, fromId: int):
        """
        Receive messages from the specified directory in the Bao vault.
        """
        r = lib.bao_receive(self.hnd, e8(dir), since, fromId)
        messages = consume(r)
        return [Message(**m) for m in messages]
    def download(self, dir: str, message: dict, attachment: int, dest: str):
        """
        Download a file from the specified message in the Bao vault.
        """
        r = lib.bao_download(self.hnd, e8(dir), j8(message), attachment, e8(dest))
        return consume(r)
    
    def __repr__(self) -> str:
        return self.URL
    
class Rows:
    def __init__(self, hnd: int):
        self.hnd = hnd
    
    def __iter__(self):
        return self
    
    def __next__(self):
        r = lib.bao_nextRow(self.hnd)
        values = consume(r)
        if values is None:
            raise StopIteration
        return values

class SqlLayer:
    def __init__(self, hnd: int):
        self.hnd = hnd
        
    def query(self, query: str, args: dict):
        r = lib.bao_query(self.hnd, e8(query), j8(args))
        consume(r)
        return Rows(r.hnd)
    
    def exec(self, query: str, args: dict):
        r = lib.bao_exec(self.hnd, e8(query), j8(args))
        return consume(r)
    
    def sync(self):
        r = lib.bao_sync(self.hnd)
        return consume(r)
    
    def rollback(self):
        r = lib.bao_rollback(self.hnd)
        return consume(r)


def test():
    #set_bao_log_level("debug")
    temp_dir = tempfile.gettempdir()
    dbfile = f"{temp_dir}/bao.db"
    if os.path.exists(dbfile):
        os.remove(dbfile)
    db = DB(dbfile)
    url = f"file://{temp_dir}/test_bao"
    alice = newPrivateID()
    vault = Bao.create(db, alice, url)
    vault.set_access(Groups.user, {publicID(alice): Access.write+Access.read})
    print(vault.get_access(Groups.user))
    
    vault = Bao.open(db, alice, publicID(alice), url)
    vault.write_data("data", Groups.user, b"hello")
    ls = vault.read_dir("", 0, 0, 10)
    print(ls)
    
    data = vault.read_data("data")
    print(data)

    vault.send("mailbox", Groups.user, Message("hello", "world"))
    print(vault.receive("mailbox", 0, 0))

    ddl = """-- INIT 
        CREATE TABLE test (id INT PRIMARY KEY, name TEXT)"""
    sl = vault.sqlLayer("vdb", Groups.user, {ddl: 1.0})   
    sl.exec("INSERT INTO test VALUES (:id, :name)", {"id": 1, "name": "hello"})
    sl.exec("INSERT INTO test VALUES (:id, :name)", {"id": 2, "name": "world"})
    rows = sl.query("SELECT * FROM test", {})
    for row in rows:
        print("Row: ", row)
    
    
    
    
