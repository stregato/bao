import 'dart:typed_data';

import 'package:bao/src/fileinfo.dart';
import 'package:bao/src/bindings.dart';
import 'message.dart';
import 'bao_ql.dart';
import 'db.dart';
import 'identity.dart';
import 'loader.dart';
import 'storage.dart';

typedef Access = int;
const int accessRead = 1;
const int accessWrite = 2;
const int accessReadWrite = accessRead | accessWrite;
const int accessAdmin = 4;

typedef Accesses = Map<PublicID, Access>;
typedef Group = String;

const Group users = 'users'; // Represents a group for normal users
const Group admins =
    'admins'; // Represents a group for users who can grant or revoke access
const Group public = 'public'; // Represents a group for public access
const Group sql =
    'sql'; // Represents a group for SQL operations. Using a specific group improves performance
const Group blockchain =
    '#blockchain'; // Represents a group for blockchain sync operations
const Group cleanup =
    '#cleanup'; // Represents a group for cleanup operations where older files are removed

typedef OpenOptions = int;

typedef RWOptions = int;
const int asyncOperation = 1; // Perform the operation asynchronously
const int scheduledOperation = 2; // Schedule the operation for later

class AccessChange {
  Group group = '';
  Access access = 0;
  PublicID userId = '';

  AccessChange(this.group, this.access, this.userId);

  Map<String, dynamic> toJson() {
    return {
      'group': group,
      'access': access,
      'userId': userId,
    };
  }
}

/// Represents a Bao instance with its handle, author, and URL.
/// Provides methods to create, open, close, and manage the vault.
class Bao {
  int hnd = 0;
  String id = '';
  PrivateID userId = '';
  PublicID userPublicId = '';
  String url = '';
  PublicID author = '';
  Map<String, dynamic> config = {};
  StoreConfig storeConfig = const StoreConfig();

  static Bao none = Bao();

  static fromResult(Result res) {
    var s = Bao();
    s.hnd = res.handle;
    var m = res.map;
    s.id = m['id'] ?? '';
    s.userId = m['userId'] ?? '';
    s.userPublicId = m['userPublicId'] ?? '';
    if (m['storeConfig'] != null && m['storeConfig'] is Map) {
      final rawManifest = m['storeConfig'] as Map;
      s.storeConfig = StoreConfig.fromJson(Map<String, dynamic>.from(
          rawManifest.map((key, value) => MapEntry(key.toString(), value))));
      s.url = s.storeConfig.id;
    } else {
      s.url = m['url'] ?? '';
    }
    s.author = m['author'] ?? '';
    s.config = m['config'] ?? {};
    return s;
  }

  /// Creates a new vault with the given identity, store configuration, and settings.
  /// The settings parameter is a map of configuration options (see Go vault.Config).
  /// Returns a tuple containing the created Bao and an error message if any.
  static Future<Bao> create(DB db, PrivateID identity, StoreConfig storeConfig,
      {Map<String, dynamic> settings = const {}}) async {
    var res = await bindings.acall(
        'bao_create', [db.hnd, identity, storeConfig.toJson(), settings]);
    return fromResult(res);
  }

  /// Opens an existing vault with the given identity, store configuration, and author.
  /// The options parameter can be used to specify additional options for the vault.
  /// Returns a tuple containing the opened Bao and an error message if any.
  static Future<Bao> open(DB db, PrivateID identity, StoreConfig storeConfig,
      PublicID author) async {
    var res = await bindings
        .acall('bao_open', [db.hnd, identity, storeConfig.toJson(), author]);
    return fromResult(res);
  }

  /// Closes the vault and releases any associated resources.
  void close() async {
    var res = await bindings.acall('bao_close', [hnd]);
    res.throwIfError();
  }

  int get allocatedSize {
    var res = bindings.call('bao_getAllocatedSize', [hnd]);
    return res.integer;
  }

  /// Applies a batch of access changes and optionally flushes them immediately.
  Future<void> syncAccess(
      [List<AccessChange> changes = const [], int options = 0]) async {
    var res = await bindings.acall('bao_syncAccess', [hnd, options, changes]);
    res.throwIfError();
  }

  /// Retrieves the access permissions for a given group name.
  /// Returns a tuple containing a map of PublicID to Access and an error message if any.
  Future<Accesses> getAccess(Group group) async {
    var res = await bindings.acall('bao_getAccess', [hnd, group]);
    return res.map.map((k, v) => MapEntry<String, int>(k, v));
  }

  // Retrieves the groups and access permissions for a given user
  // Returns a tuple containing a map of Group to Access and an error message if any.
  Future<Map<Group, Access>> getGroups(String user) async {
    var res = await bindings.acall('bao_getGroups', [hnd, user]);
    return res.map.map((k, v) => MapEntry<Group, Access>(k, v));
  }

  /// Waits for the specified files (by file ID) to complete pending I/O.
  /// The [fileIds] parameter is a list of file IDs to wait for.
  Future<void> waitFiles([List<int> fileIds = const []]) async {
    var res = await bindings.acall('bao_waitFiles', [hnd, fileIds]);
    res.throwIfError();
  }

  /// Returns the list of groups in the stack
  Future<List<Group>> listGroups() async {
    var res = await bindings.acall('bao_listGroups', [hnd]);
    return res.list.map((e) => e as Group).toList();
  }

  /// Synchronizes the dirs contents of the vault and returns the list of changed files.
  /// The [groups] parameter is a list of group names to sync.
  Future<List<FileInfo>> sync([List<Group> groups = const []]) async {
    // Maps to Go export bao_sync
    var res = await bindings.acall('bao_sync', [hnd, groups]);
    return res.list.map((e) => FileInfo.fromMap(e)).toList();
  }

  /// Sets a custom attribute for the vault.
  /// The [name] parameter is the name of the attribute.
  /// The [value] parameter is the value of the attribute.
  Future<void> setAttribute(String name, String value,
      [int options = 0]) async {
    var res =
        await bindings.acall('bao_setAttribute', [hnd, options, name, value]);
    res.throwIfError();
  }

  /// Retrieves the value of a custom attribute for the vault.
  /// The [name] parameter is the name of the attribute.
  Future<String> getAttribute(String name, PublicID author) async {
    var res = await bindings.acall('bao_getAttribute', [hnd, name, author]);
    return res.string;
  }

  /// Retrieves all custom attributes for the vault.
  /// The [author] parameter is the PublicID of the author requesting the attributes.
  Future<Map<PublicID, String>> getAttributes(PublicID author) async {
    var res = await bindings.acall('bao_getAttributes', [hnd, author]);
    return res.map.map((k, v) => MapEntry<PublicID, String>(k, v));
  }

  /// Reads the directory contents of the vault.
  /// The [dir] parameter specifies the directory to read.
  /// The [since] parameter can be used to filter files modified since a specific date.
  /// The [fromId] parameter specifies the starting file ID for pagination.
  /// The [limit] parameter specifies the maximum number of files to return.
  Future<List<FileInfo>> readDir(String dir,
      {DateTime? since, int fromId = 0, int limit = 0}) async {
    final sinceSec = since == null ? 0 : (since.millisecondsSinceEpoch ~/ 1000);
    var res = await bindings
        .acall('bao_readDir', [hnd, dir, sinceSec, fromId, limit]);
    return res.list.map((e) => FileInfo.fromMap(e)).toList();
  }

  /// Get file information for the given name.
  /// Returns a tuple containing the FileInfo object and an error message if any.
  Future<FileInfo> stat(String name) async {
    var res = await bindings.acall('bao_stat', [hnd, name]);
    return FileInfo.fromMap(res.map);
  }

  /// Reads data from the vault with the given name and destination path.
  /// The options parameter can be used to specify additional options for the read operation.
  /// Returns an error if the read operation fails.
  Future<FileInfo> read(String name, String dst, {int options = 0}) async {
    var res = await bindings.acall('bao_read', [hnd, name, dst, options]);
    return FileInfo.fromMap(res.map);
  }

  /// Writes data to the vault with the given destination, group, and source path.
  /// The src parameter is the source path of the file to be written. If src is empty, it will write only the header without any content.
  /// The options parameter can be used to specify additional options for the write operation.
  Future<FileInfo> write(String dest, Group group,
      {Uint8List? attrs, String src = "", int options = 0}) async {
    attrs ??= Uint8List(0);
    var res = await bindings
        .acall('bao_write', [hnd, dest, src, group, attrs, options]);
    return FileInfo.fromMap(res.map);
  }

  /// Deletes the file with the given name from the vault.
  /// Returns an error if the deletion operation fails.
  Future<void> delete(String name, {int options = 0}) async {
    var res = await bindings.acall('bao_delete', [hnd, name, options]);
    res.throwIfError();
  }

  /// Creates a new SQL layer for the specified database.
  Future<BaoQL> baoQL(String group, DB db) async {
    var res = await bindings.acall('baoql_layer', [hnd, group, db.hnd]);
    return BaoQL(res.handle);
  }

  /// Sends a message to the specified directory and group.
  /// The [message] parameter is the message to be sent.
  ///
  Future<void> send(String dir, Group group, Message message) async {
    var res = await bindings.acall('mailbox_send', [hnd, dir, group, message]);
    res.throwIfError();
  }

  /// Receives messages from the specified directory.
  /// The [since] parameter can be used to filter messages received since a specific date.
  /// The [fromId] parameter specifies the starting message ID for pagination.
  Future<List<Message>> receive(String dir,
      {DateTime? since, int fromId = 0}) async {
    var res = await bindings.acall('mailbox_receive',
        [hnd, dir, since?.millisecondsSinceEpoch ?? 0, fromId]);
    return res.list.map((x) => Message.fromJson(x)).toList();
  }

  /// Downloads the specified attachment of a message to the destination path.
  Future<void> download(
      String dir, Message message, int attachmentIndex, String dest) async {
    var res = await bindings
        .acall('mailbox_download', [hnd, dir, message, attachmentIndex, dest]);
    res.throwIfError();
  }
}
