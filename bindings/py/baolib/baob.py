import ctypes
import json
from .baod import *

lib = load_lib()

# Global helpers exported by the Go library
lib.bao_setLogLevel.argtypes = [ctypes.c_char_p]
lib.bao_setLogLevel.restype = Result
lib.bao_core_setHttpLog.argtypes = [ctypes.c_char_p]
lib.bao_core_setHttpLog.restype = Result
lib.bao_core_getRecentLog.argtypes = [ctypes.c_int]
lib.bao_core_getRecentLog.restype = Result
lib.bao_snapshot.argtypes = []
lib.bao_snapshot.restype = Result
lib.bao_test.argtypes = []
lib.bao_test.restype = Result

# Identity
lib.bao_security_newPrivateID.argtypes = []
lib.bao_security_newPrivateID.restype = Result
lib.bao_security_publicID.argtypes = [ctypes.c_char_p]
lib.bao_security_publicID.restype = Result
lib.bao_security_newKeyPair.argtypes = []
lib.bao_security_newKeyPair.restype = Result
lib.bao_security_decodeID.argtypes = [ctypes.c_char_p]
lib.bao_security_decodeID.restype = Result
lib.bao_security_ecEncrypt.argtypes = [ctypes.c_char_p, Data]
lib.bao_security_ecEncrypt.restype = Result
lib.bao_security_ecDecrypt.argtypes = [ctypes.c_char_p, Data]
lib.bao_security_ecDecrypt.restype = Result
lib.bao_security_aesEncrypt.argtypes = [ctypes.c_char_p, Data, Data]
lib.bao_security_aesEncrypt.restype = Result
lib.bao_security_aesDecrypt.argtypes = [ctypes.c_char_p, Data, Data]
lib.bao_security_aesDecrypt.restype = Result

# DB
lib.bao_db_open.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_db_open.restype = Result
lib.bao_db_close.argtypes = [ctypes.c_longlong]
lib.bao_db_close.restype = Result
lib.bao_db_query.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_db_query.restype = Result
lib.bao_db_exec.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_db_exec.restype = Result
lib.bao_db_fetch.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
lib.bao_db_fetch.restype = Result
lib.bao_db_fetch_one.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_db_fetch_one.restype = Result

# Store
lib.bao_store_open.argtypes = [ctypes.c_char_p]
lib.bao_store_open.restype = Result
lib.bao_store_close.argtypes = [ctypes.c_longlong]
lib.bao_store_close.restype = Result
lib.bao_store_readDir.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_store_readDir.restype = Result
lib.bao_store_stat.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_store_stat.restype = Result
lib.bao_store_delete.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_store_delete.restype = Result

# Vault
lib.bao_vault_create.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong, ctypes.c_char_p]
lib.bao_vault_create.restype = Result
lib.bao_vault_open.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_vault_open.restype = Result
lib.bao_vault_close.argtypes = [ctypes.c_longlong]
lib.bao_vault_close.restype = Result
lib.bao_vault_syncAccess.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_char_p]
lib.bao_vault_syncAccess.restype = Result
lib.bao_vault_getAccesses.argtypes = [ctypes.c_longlong]
lib.bao_vault_getAccesses.restype = Result
lib.bao_vault_getAccess.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_vault_getAccess.restype = Result
lib.bao_vault_waitFiles.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_vault_waitFiles.restype = Result
lib.bao_vault_sync.argtypes = [ctypes.c_longlong]
lib.bao_vault_sync.restype = Result
lib.bao_vault_setAttribute.argtypes = [ctypes.c_longlong, ctypes.c_int, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_vault_setAttribute.restype = Result
lib.bao_vault_getAttribute.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_vault_getAttribute.restype = Result
lib.bao_vault_getAttributes.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_vault_getAttributes.restype = Result
lib.bao_vault_readDir.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong, ctypes.c_int]
lib.bao_vault_readDir.restype = Result
lib.bao_vault_stat.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_vault_stat.restype = Result
lib.bao_vault_getGroup.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_vault_getGroup.restype = Result
lib.bao_vault_getAuthor.argtypes = [ctypes.c_longlong, ctypes.c_char_p]
lib.bao_vault_getAuthor.restype = Result
lib.bao_vault_read.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_longlong]
lib.bao_vault_read.restype = Result
lib.bao_vault_write.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, Data, ctypes.c_longlong]
lib.bao_vault_write.restype = Result
lib.bao_vault_delete.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_int]
lib.bao_vault_delete.restype = Result
lib.bao_vault_allocatedSize.argtypes = [ctypes.c_longlong]
lib.bao_vault_allocatedSize.restype = Result

# Replica
lib.bao_replica_open.argtypes = [ctypes.c_longlong, ctypes.c_int]
lib.bao_replica_open.restype = Result
lib.bao_replica_exec.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_replica_exec.restype = Result
lib.bao_replica_query.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_replica_query.restype = Result
lib.bao_replica_fetch.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
lib.bao_replica_fetch.restype = Result
lib.bao_replica_fetchOne.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_replica_fetchOne.restype = Result
lib.bao_replica_sync.argtypes = [ctypes.c_longlong]
lib.bao_replica_sync.restype = Result
lib.bao_replica_cancel.argtypes = [ctypes.c_longlong]
lib.bao_replica_cancel.restype = Result
lib.bao_replica_current.argtypes = [ctypes.c_longlong]
lib.bao_replica_current.restype = Result
lib.bao_replica_next.argtypes = [ctypes.c_longlong]
lib.bao_replica_next.restype = Result
lib.bao_replica_closeRows.argtypes = [ctypes.c_longlong]
lib.bao_replica_closeRows.restype = Result

# Mailbox
lib.bao_mailbox_send.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_mailbox_send.restype = Result
lib.bao_mailbox_receive.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong]
lib.bao_mailbox_receive.restype = Result
lib.bao_mailbox_download.argtypes = [ctypes.c_longlong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p]
lib.bao_mailbox_download.restype = Result


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
