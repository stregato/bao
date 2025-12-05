import ctypes
import platform
import os
import json
from dataclasses import dataclass, asdict
from datetime import datetime
import pytz
import pkg_resources
import base64
import os
import platform
import ctypes

def load_lib():
    # Directly using normalized architecture names in the paths dictionary
    paths = {
        ("Windows", "x86_64"): "bao.dll",
        ("Linux", "x86_64"): "libbao.so",
        ("Darwin", "x86_64"): "libbao.dylib",
        ("Darwin", "arm64"): "libbao.dylib",
        ("Linux", "arm64"): "libbao.so",
        ("Android", "arm64"): "libbao.so",
    }

    os_name = platform.system().lower()
    arch = platform.machine().lower()

    # Normalize architecture names to match dictionary keys
    arch_map = {
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }
    ext_map = {
        "windows": ".dll",
        "linux": ".so",
        "darwin": ".dylib",
    }
    arch = arch_map.get(arch, arch)
    ext = ext_map.get(os_name)
    name = "bao" if os_name == "windows" else "libbao"
    name = f"{name}_{arch}{ext}"

    # Search for the library in the _libs folder or in the ../../build/architecture folder
    package_dir = os.path.dirname(os.path.abspath(__file__))
    search_paths = [
        os.path.abspath(os.path.join(package_dir, '_libs', name)),
        os.path.abspath(os.path.join(package_dir, f'../../build/{os_name}', name))
    ]

    # Find the library in the specified paths
    for path in search_paths:
        if os.path.exists(path):
            lib = ctypes.CDLL(path)
            lib.free.argtypes = [ctypes.c_void_p]
            lib.free.restype = None
            return lib

    raise RuntimeError(f"Library not found in any of the search paths: {search_paths}")
def json_serial(obj):
    """JSON serializer for objects not serializable by default json code"""
    if isinstance(obj, datetime):
        if not obj.tzinfo:
            obj = pytz.utc.localize(obj)
        return obj.isoformat()
    raise TypeError("Type %s not serializable" % type(obj))

def e8(s):
    """encode utf-8 string"""
    return ctypes.c_char_p(s.encode("utf-8"))

def j8(s):
    """encode json object to utf-8 string"""
    return json.dumps(s).encode("utf-8")

def o8(o):
    """encode dataclass object to utf-8 string"""
    if hasattr(o, "toJson"):
        return e8(o.toJson())
    else:
        return json.dumps(asdict(o), default=json_serial).encode("utf-8")

class Result(ctypes.Structure):
    _fields_ = [("ptr", ctypes.c_void_p), ("len", ctypes.c_size_t),("hnd", ctypes.c_ulonglong), ("err", ctypes.c_char_p)]

    def __repr__(self):
        return f"Result(ptr={self.ptr}, len={self.len}, hnd={self.hnd}, err={self.err})"
    
class Data(ctypes.Structure):
    _fields_ = [
        ("ptr", ctypes.c_void_p),
        ("len", ctypes.c_size_t)
    ]

    def __repr__(self):
        return f"Data(ptr={self.ptr}, len={self.len})"

    @staticmethod
    def from_byte_array(byte_array):
        # Ensure byte_array is a bytes object
        if not isinstance(byte_array, (bytes, bytearray)):
            raise TypeError("byte_array must be a bytes or bytearray object")
        
        # Get the length of the byte array
        length = len(byte_array)
        
        # Allocate memory in the C heap
        ptr = ctypes.cast(ctypes.create_string_buffer(byte_array), ctypes.c_void_p)
        
        # Create and initialize the Data struct
        data = Data(ptr=ptr, len=length)
        
        return data