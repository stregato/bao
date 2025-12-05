import ctypes
import platform
import os
import json
from dataclasses import dataclass, asdict, field
from datetime import timedelta, datetime
from typing import Dict, Optional, List, Any


time0 = datetime.min
Users = Dict[str, int]


@dataclass
class CreateOptions:
    """
    Represents options for creating a safe.

    Attributes:
        wipe (bool): True if the safe should be wiped before creating it.
        description (str): Description of the safe.
        change_log_watch (float): The period for watching changes in the change log in seconds.
        replica_watch (float): The period for synchronizing replicas in seconds.
        quota (int): The maximum size of the safe in bytes.
        quota_group (str): The common prefix for the safes that share the quota.
    """
    storeName: str = ""
    quota: int = 0
    wipe: bool = False
    description: str = ""
    change_log_watch: float = 0.0
    replica_watch: float = 0.0

@dataclass
class OpenOptions:
    """
    Represents options for opening a resource.

    Attributes:
        force_create (bool): Whether to force creation.
        sync_period (float): The synchronization period in seconds.
        adaptive_sync (bool): Whether adaptive synchronization is enabled.
    """

    force_create: bool = False
    sync_period: float = 0.0
    adaptive_sync: bool = False

@dataclass
class ListOptions:
    """
    Represents a set of options for listing files.

    Attributes:
        name (str): Filter on the file name.
        depth (int): Level of depth into subfolders in the directory. -1 means infinite.
        suffix (str): Filter on the file suffix.
        contentType (str): Filter on the content type.
        bodyId (int): Filter on the body ID.
        tags (List[str]): Filter on the tags.
        before (time.time): Filter on the modification time.
        after (time.time): Filter on the modification time.
        knownSince (time.time): Filter on the sync time.
        offset (int): Offset of the first file to return.
        limit (int): Maximum number of files to return.
        includeDeleted (bool): Include deleted files.
        creator (str): Filter on the creator.
        noPrivate (bool): Ignore private files.
        privateId (str): Filter on private files either created by the current user or the specified user.
        prefetch (bool): Prefetch the file bodies.
        errorIfNotExist (bool): Return an error if the directory does not exist. Otherwise, return an empty list.
        orderBy (str): Order by name or modTime. Default is name.
        reverseOrder (bool): Order descending when true. Default is false.
    """

    name: str = ""              # Filter on the file name
    depth: int = -1             # Level of depth into subfolders in the directory. -1 means infinite
    suffix: str = ""            # Filter on the file suffix
    contentType: str = ""       # Filter on the content type
    bodyId: int = 0             # Filter on the body ID
    tags: List[str] = None        # Filter on the tags
    before: datetime = time0        # Filter on the modification time
    after: datetime = time0         # Filter on the modification time
    knownSince: datetime = time0    # Filter on the sync time
    offset: int = 0             # Offset of the first file to return
    limit: int = 100            # Maximum number of files to return
    includeDeleted: bool = False  # Include deleted files
    creator: str = ""           # Filter on the creator
    noPrivate: bool = False     # Ignore private files
    privateId: str = ""         # Filter on private files either created by the current user or the specified user
    prefetch: bool = False      # Prefetch the file bodies
    errorIfNotExist: bool = False   # Return an error if the directory does not exist. Otherwise, return an empty list
    orderBy: str = ""               # Order by name or modTime. Default is name
    reverseOrder: bool = False      # Order descending when true. Default is false

@dataclass
class ListDirsOptions:
    """
    Represents options for listing directories.

    Attributes:
        depth (int): Level of depth into subfolders in the directory. -1 means infinite.
        errorIfNotExist (bool): Return an error if the directory does not exist. Otherwise, return an empty list.
    """

    depth: int = -1
    errorIfNotExist: bool = False

from dataclasses import dataclass
from typing import List, Dict, Any

@dataclass
class PutOptions:
    """
    Represents options for putting a file.

    Attributes:
        progress (list[int]): Progress channel.
        replace (bool): Replace all other files with the same name.
        replace_id (int): Replace the file with the specified ID.
        tags (List[str]): Tags associated with the file.
        thumbnail (bytes): Thumbnail associated with the file.
        auto_thumbnail (bool): Generate a thumbnail from the file.
        content_type (str): Content type of the file.
        zip (bool): Zip the file if it is smaller than 64MB.
        meta (Dict[str, Any]): Metadata associated with the file.
        source (str): Track the source of the file as the download location.
        private (str): ID of the target user in case of a private message.
    """

    replace: bool = False
    replace_id: int = 0
    tags: List[str] = None
    thumbnail: bytes = None
    auto_thumbnail: bool = False
    content_type: str = ""
    zip: bool = False
    meta: Dict[str, Any] = None
    source: str = ""
    private: str = ""


@dataclass
class Range:
    """
    Represents a range of integers.

    Attributes:
        fromValue (int): The starting value of the range.
        toValue (int): The ending value of the range.
    """

    fromValue: int
    toValue: int


@dataclass
class GetOptions:
    """
    Represents options for getting a file.

    Attributes:
        destination (str): Track the file is downloaded to the specified destination. The file must be closed within a second.
        fileId (int): Get the file with the specified body ID.
        noCache (bool): Do not cache the file.
        cacheExpire (float): Cache expiration time in seconds.
        fileRange (Optional[Range]): Range of bytes to read.
    """

    destination: str = ""
    fileId: int = 0
    noCache: bool = False
    cacheExpire: float = 0.0
    fileRange: Optional[Range] = None


@dataclass
class SetUsersOptions:
    """
    Represents options for setting users.

    Attributes:
        replaceUsers (bool): Replace users.
        alignDelay (float): Align delay in seconds.
        syncAlign (bool): Synchronize alignment.
    """

    replaceUsers: bool = False
    alignDelay: float = 0.0
    syncAlign: bool = False