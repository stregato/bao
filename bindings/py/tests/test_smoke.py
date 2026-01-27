import os
import platform

import pytest


def _native_library_present() -> bool:
    os_name = platform.system().lower()
    arch = platform.machine().lower()

    arch_map = {
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
        "x86_64": "amd64",
    }
    ext_map = {
        "windows": ".dll",
        "linux": ".so",
        "darwin": ".dylib",
    }
    arch = arch_map.get(arch, arch)
    ext = ext_map.get(os_name)
    if ext is None:
        return False

    base = "bao" if os_name == "windows" else "libbao"
    name = f"{base}_{arch}{ext}"

    package_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "baolib"))
    search_paths = [
        os.path.join(package_dir, "_libs", name),
        os.path.abspath(os.path.join(package_dir, f"../../../build/{os_name}", name)),
    ]
    return any(os.path.exists(path) for path in search_paths)


def test_import_and_ids_smoke():
    if not _native_library_present():
        pytest.skip("Native Bao library not found for this platform.")

    import baolib

    private_id = baolib.newPrivateID()
    public_id = baolib.publicID(private_id)

    assert isinstance(private_id, str)
    assert isinstance(public_id, str)
    assert private_id
    assert public_id
