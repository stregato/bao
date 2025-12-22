import ctypes
import json
from .baod import *

lib = load_lib()

# Global helpers exported by the Go library
lib.bao_setLogLevel.argtypes = [ctypes.c_char_p]
lib.bao_setLogLevel.restype = Result
lib.bao_setHttpLog.argtypes = [ctypes.c_char_p]
lib.bao_setHttpLog.restype = Result
lib.bao_getRecentLog.argtypes = [ctypes.c_int]
lib.bao_getRecentLog.restype = Result
lib.bao_snapshot.argtypes = []
lib.bao_snapshot.restype = Result
lib.bao_test.argtypes = []
lib.bao_test.restype = Result

# Identity
lib.bao_newPrivateID.argtypes = []
lib.bao_newPrivateID.restype = Result
lib.bao_publicID.argtypes = [ctypes.c_char_p]
lib.bao_publicID.restype = Result
lib.bao_decodeID.argtypes = [ctypes.c_char_p]
lib.bao_decodeID.restype = Result
lib.bao_ecEncrypt.argtypes = [ctypes.c_char_p, Data]
lib.bao_ecEncrypt.restype = Result
lib.bao_ecDecrypt.argtypes = [ctypes.c_char_p, Data]
lib.bao_ecDecrypt.restype = Result
lib.bao_aesEncrypt.argtypes = [ctypes.c_char_p, Data, Data]
lib.bao_aesEncrypt.restype = Result
lib.bao_aesDecrypt.argtypes = [ctypes.c_char_p, Data, Data]
lib.bao_aesDecrypt.restype = Result

# DB
lib.bao_openDB.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_openDB.restype = Result
lib.bao_closeDB.argtypes = [ctypes.c_longlong]
lib.bao_closeDB.restype = Result
lib.bao_dbQuery.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_dbQuery.restype = Result
lib.bao_dbExec.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_dbExec.restype = Result
lib.bao_dbFetch.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
lib.bao_dbFetch.restype = Result
lib.bao_dbFetchOne.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_dbFetchOne.restype = Result

# Bao
lib.bao_create.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_create.restype = Result
lib.bao_open.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_open.restype = Result
lib.bao_close.argtypes = [ctypes.c_longlong]
lib.bao_close.restype = Result
lib.bao_syncAccess.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_char_p]
lib.bao_syncAccess.restype = Result
lib.bao_getAccess.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_getAccess.restype = Result
lib.bao_getGroups.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_getGroups.restype = Result
lib.bao_waitFiles.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_waitFiles.restype = Result
lib.bao_listGroups.argtypes = [ctypes.c_longlong]
lib.bao_listGroups.restype = Result
lib.bao_sync.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_sync.restype = Result
lib.bao_setAttribute.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_setAttribute.restype = Result
lib.bao_getAttribute.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_getAttribute.restype = Result
lib.bao_getAttributes.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_getAttributes.restype = Result
lib.bao_readDir.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong, ctypes.c_int]
lib.bao_readDir.restype = Result
lib.bao_stat.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_stat.restype = Result
lib.bao_read.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_longlong]
lib.bao_read.restype = Result
lib.bao_write.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p, Data, ctypes.c_longlong]
lib.bao_write.restype = Result
lib.bao_delete.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_int]
lib.bao_delete.restype = Result
lib.bao_allocatedSize.argtypes = [ctypes.c_longlong]
lib.bao_allocatedSize.restype = Result

# SQL layer
lib.baoql_layer.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_int]
lib.baoql_layer.restype = Result
lib.baoql_exec.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.baoql_exec.restype = Result
lib.baoql_query.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.baoql_query.restype = Result
lib.baoql_fetch.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
lib.baoql_fetch.restype = Result
lib.baoql_fetchOne.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.baoql_fetchOne.restype = Result
lib.baoql_sync_tables.argtypes = [ctypes.c_longlong]
lib.baoql_sync_tables.restype = Result
lib.baoql_cancel.argtypes = [ctypes.c_longlong]
lib.baoql_cancel.restype = Result
lib.baoql_current.argtypes = [ctypes.c_longlong]
lib.baoql_current.restype = Result
lib.baoql_next.argtypes = [ctypes.c_longlong]
lib.baoql_next.restype = Result
lib.baoql_closeRows.argtypes = [ctypes.c_longlong]
lib.baoql_closeRows.restype = Result

# Mailbox
lib.mailbox_send.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p]
lib.mailbox_send.restype = Result
lib.mailbox_receive.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong]
lib.mailbox_receive.restype = Result
lib.mailbox_download.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p]
lib.mailbox_download.restype = Result


def consume(r, returnBytes: bool = False):
    """Consume a Result by decoding the JSON payload and freeing the buffer."""
    try:
        if r.err:
            raise Exception(r.err.decode("utf-8"))

        if not r.ptr:
            return None

        byte_array = (ctypes.c_ubyte * r.len).from_address(r.ptr)
        byte_data = bytes(byte_array)
        return byte_data if returnBytes else json.loads(byte_data)
    finally:
        if r.ptr:
            lib.free(r.ptr)
            r.ptr = None
