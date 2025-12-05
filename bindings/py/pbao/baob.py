import ctypes
import platform
import os
import json
from dataclasses import dataclass, asdict
from datetime import datetime
from .baod import *

lib = load_lib()

# Match the functions exported in export.go
lib.bao_setLogLevel.argtypes = [ctypes.c_char_p]
lib.bao_setLogLevel.restype = Result

lib.bao_openDB.argtypes = [ctypes.c_char_p]
lib.bao_openDB.restype = Result

lib.bao_closeDB.argtypes = [ctypes.c_ulonglong]
lib.bao_closeDB.restype = Result

lib.bao_newPrivateID.argtypes = []
lib.bao_newPrivateID.restype = Result

lib.bao_publicID.argtypes = [ctypes.c_char_p]
lib.bao_publicID.restype = Result

lib.bao_create.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_ulonglong]
lib.bao_create.restype = Result

lib.bao_open.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_ulonglong]
lib.bao_open.restype = Result

lib.bao_close.argtypes = [ctypes.c_ulonglong]
lib.bao_close.restype = Result

lib.bao_setRetention.argtypes = [ctypes.c_ulonglong, ctypes.c_longlong, ctypes.c_longlong]
lib.bao_setRetention.restype = Result

lib.bao_setAccess.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_setAccess.restype = Result

lib.bao_getAccess.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p]
lib.bao_getAccess.restype = Result

lib.bao_readDir.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong, ctypes.c_int]
lib.bao_readDir.restype = Result

lib.bao_getGroup.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p]
lib.bao_getGroup.restype = Result

lib.bao_getAuthor.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p]
lib.bao_getAuthor.restype = Result

lib.bao_readFile.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_ulonglong]
lib.bao_readFile.restype = Result

lib.bao_writeFile.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_ulonglong]   
lib.bao_writeFile.restype = Result

lib.bao_readData.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_ulonglong]
lib.bao_readData.restype = Result

lib.bao_writeData.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, Data, ctypes.c_ulonglong]
lib.bao_writeData.restype = Result

lib.bao_sqlLayer.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_sqlLayer.restype = Result

lib.bao_exec.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_exec.restype = Result

lib.bao_query.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_query.restype = Result

lib.bao_nextRow.argtypes = [ctypes.c_ulonglong]
lib.bao_nextRow.restype = Result

lib.bao_sync.argtypes = [ctypes.c_ulonglong]
lib.bao_sync.restype = Result

lib.bao_rollback.argtypes = [ctypes.c_ulonglong]
lib.bao_rollback.restype = Result

lib.bao_send.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p]
lib.bao_send.restype = Result

lib.bao_receive.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_longlong, ctypes.c_longlong]
lib.bao_receive.restype = Result

lib.bao_download.argtypes = [ctypes.c_ulonglong, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p]
lib.bao_download.restype = Result

def consume(r, returnBytes=False):
    try:
        if r.err:
            raise Exception(r.err.decode("utf-8"))
        
        if not r.ptr:
            return None
        
        # Interpret ptr as a byte array of length len
        byte_array = (ctypes.c_ubyte * r.len).from_address(r.ptr)
        byte_data = bytes(byte_array)
        
        # Convert byte array to JSON object
        return byte_data if returnBytes else json.loads(byte_data)
    
    finally:
        # Free the allocated memory for ptr and err
        if r.ptr:
            lib.free(r.ptr)
            r.ptr = None