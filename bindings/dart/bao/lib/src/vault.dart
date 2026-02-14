import 'dart:typed_data';

import 'package:bao/src/fileinfo.dart';
import 'package:bao/src/bindings.dart';
import 'config.dart';
import 'db.dart';
import 'identity.dart';
import 'loader.dart';
import 'store.dart';

typedef Access = int;
const int accessRead = 1;
const int accessWrite = 2;
const int accessAdmin = 4;
const int accessReadWrite = accessRead | accessWrite;
const int accessAdminReadWrite = accessAdmin | accessReadWrite;

typedef Accesses = Map<PublicID, Access>;

typedef OpenOptions = int;

typedef RWOptions = int;
const int asyncOperation = 1; // Perform the operation asynchronously
const int scheduledOperation = 2; // Schedule the operation for later

class AccessChange {
  PublicID userId = PublicID();
  Access access = 0;

  AccessChange(this.userId, this.access);

  Map<String, dynamic> toJson() {
    return {
      'userId': userId.toString(),
      'access': access,
    };
  }
}

typedef Realm = String;
const Realm users = 'users';
const Realm home = 'home';
const Realm all = 'all';

/// Represents a Bao instance with its handle, author, and URL.
/// Provides methods to create, open, close, and manage the vault.
class Vault {
  int hnd = 0;
  String id = '';
  late PrivateID userSecret;
  late PublicID userId;
  late PublicID author;
  Realm realm = '';
  String url = '';
  Map<String, dynamic> config = {};
  StoreConfig storeConfig = const StoreConfig();

  static Vault none = Vault();

  static fromResult(Result res) {
    var s = Vault();
    s.hnd = res.handle;
    var m = res.map;
    s.id = m['id'] ?? '';
    s.userSecret = PrivateID(m['userSecret']);
    s.userId = PublicID(m['userId']);
    if (m['storeConfig'] != null && m['storeConfig'] is Map) {
      final rawManifest = m['storeConfig'] as Map;
      s.storeConfig = StoreConfig.fromJson(Map<String, dynamic>.from(
          rawManifest.map((key, value) => MapEntry(key.toString(), value))));
      s.url = s.storeConfig.id;
    } else {
      s.url = m['url'] ?? '';
    }
    s.author = PublicID(m['author'] as String? ?? '');
    s.config = m['config'] ?? {};
    s.realm = m['realm'] ?? '';
    return s;
  }

  /// Creates a new vault with the given identity, store configuration, and config.
  /// The config parameter is a map of configuration options (see Go vault.Config).
  /// Returns a tuple containing the created Bao and an error message if any.
  static Future<Vault> create(
      Realm realm, PrivateID identity, Store store, DB db,
      {Config? config}) async {
    var configMap = config?.toJson() ?? {};
    var res = await bindings.acall(
        'bao_vault_create', [realm, identity, store.hnd, db.hnd, configMap]);
    return fromResult(res);
  }

  /// Opens an existing vault with the given identity and author.
  /// Returns the opened vault or throws if opening fails.
  static Future<Vault> open(
      Realm realm, PrivateID identity, PublicID author, Store store, DB db) async {
    var res = await bindings.acall(
        'bao_vault_open', [realm, identity, author, store.hnd, db.hnd]);
    return fromResult(res);
  }

  /// Closes the vault and releases any associated resources.
  void close() async {
    var res = await bindings.acall('bao_vault_close', [hnd]);
    res.throwIfError();
  }

  int get allocatedSize {
    var res = bindings.call('bao_vault_allocatedSize', [hnd]);
    return res.integer;
  }

  /// Applies a batch of access changes and optionally flushes them immediately.
  Future<void> syncAccess(
      [List<AccessChange> changes = const [], int options = 0]) async {
    var res =
        await bindings.acall('bao_vault_syncAccess', [hnd, options, changes]);
    res.throwIfError();
  }

  /// Retrieves the access permissions for the realm.
  Future<Accesses> getAccesses() async {
    var res = await bindings.acall('bao_vault_getAccesses', [hnd]);
    return res.map.map((k, v) => MapEntry<PublicID, int>(PublicID(k), v));
  }

  Future<Access> getAccess(PublicID userId) async {
    var res = await bindings.acall('bao_vault_getAccess', [hnd, userId]);
    return res.integer;
  }

  /// Waits for the specified files (by file ID) to complete pending I/O.
  /// The [timeoutMs] parameter is the timeout in milliseconds (0 for no timeout).
  /// The [fileIds] parameter is a list of file IDs to wait for.
  /// Returns the list of files that completed I/O operations.
  Future<List<FileInfo>> waitFiles([int timeoutMs = 0, List<int> fileIds = const []]) async {
    var res = await bindings.acall('bao_vault_waitFiles', [hnd, timeoutMs, fileIds]);
    res.throwIfError();
    if (res.data.isEmpty) {
      return [];
    }
    return res.list.map((e) => FileInfo.fromMap(e)).toList();
  }

  /// Returns the list of groups in the stack
  /// Synchronizes the dirs contents of the vault and returns the list of changed files.
  Future<List<FileInfo>> sync() async {
    var res = await bindings.acall('bao_vault_sync', [hnd]);
    return res.list.map((e) => FileInfo.fromMap(e)).toList();
  }

  /// Sets a custom attribute for the vault.
  /// The [name] parameter is the name of the attribute.
  /// The [value] parameter is the value of the attribute.
  Future<void> setAttribute(String name, String value,
      [int options = 0]) async {
    var res = await bindings
        .acall('bao_vault_setAttribute', [hnd, options, name, value]);
    res.throwIfError();
  }

  /// Retrieves the value of a custom attribute for the vault.
  /// The [name] parameter is the name of the attribute.
  Future<String> getAttribute(String name, PublicID author) async {
    var res =
        await bindings.acall('bao_vault_getAttribute', [hnd, name, author]);
    return res.string;
  }

  /// Retrieves all custom attributes for the vault.
  /// The [author] parameter is the PublicID of the author requesting the attributes.
  Future<Map<PublicID, String>> getAttributes(PublicID author) async {
    var res = await bindings.acall('bao_vault_getAttributes', [hnd, author]);
    return res.map.map((k, v) => MapEntry<PublicID, String>(PublicID(k), v));
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
        .acall('bao_vault_readDir', [hnd, dir, sinceSec, fromId, limit]);
    return res.list.map((e) => FileInfo.fromMap(e)).toList();
  }

  /// Get file information for the given name.
  /// Returns a tuple containing the FileInfo object and an error message if any.
  Future<FileInfo> stat(String name) async {
    var res = await bindings.acall('bao_vault_stat', [hnd, name]);
    return FileInfo.fromMap(res.map);
  }

  /// Reads data from the vault with the given name and destination path.
  /// The options parameter can be used to specify additional options for the read operation.
  /// Returns an error if the read operation fails.
  Future<FileInfo> read(String name, String dst, {int options = 0}) async {
    var res = await bindings.acall(
        'bao_vault_read', [hnd, name, dst, options]);
    return FileInfo.fromMap(res.map);
  }

  /// Writes data to the vault with the given destination, group, and source path.
  /// The src parameter is the source path of the file to be written. If src is empty, it will write only the header without any content.
  /// The options parameter can be used to specify additional options for the write operation.
  Future<FileInfo> write(String dest,
      {Uint8List? attrs, String src = "", int options = 0}) async {
    attrs ??= Uint8List(0);
    var res = await bindings
        .acall('bao_vault_write', [hnd, dest, src, attrs, options]);
    return FileInfo.fromMap(res.map);
  }

  /// Deletes the file with the given name from the vault.
  /// Returns an error if the deletion operation fails.
  Future<void> delete(String name, {int options = 0}) async {
    var res = await bindings.acall('bao_vault_delete', [hnd, name, options]);
    res.throwIfError();
  }

  /// Returns the author of the specified file in the vault.
  /// The [name] parameter specifies the file name.
  Future<String> getAuthor(String name) async {
    var res = await bindings.acall('bao_vault_getAuthor', [hnd, name]);
    return res.string;
  }

  /// Returns the versions of the specified file in the vault.
  /// The [name] parameter specifies the file name.
  Future<List<FileInfo>> versions(String name) async {
    var res = await bindings.acall('bao_vault_versions', [hnd, name]);
    return res.list.map((e) => FileInfo.fromMap(e)).toList();
  }

  /// Waits for updates on the vault (e.g., new files synced).
  /// The [timeoutMs] parameter specifies the timeout in milliseconds (0 for no timeout).
  /// Returns true if updates arrived before the timeout, false if timeout expired.
  Future<bool> waitUpdates([int timeoutMs = 0]) async {
    var res = await bindings.acall('bao_vault_waitUpdates', [hnd, timeoutMs]);
    return res.boolean;
  }
}
