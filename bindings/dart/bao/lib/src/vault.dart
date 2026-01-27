import 'dart:typed_data';

import 'package:bao/src/fileinfo.dart';
import 'package:bao/src/bindings.dart';
import 'message.dart';
import 'mailbox.dart';
import 'bao_ql.dart';
import 'db.dart';
import 'identity.dart';
import 'loader.dart';
import 'store.dart';

typedef Access = int;
const int accessRead = 1;
const int accessWrite = 2;
const int accessReadWrite = accessRead | accessWrite;
const int accessAdmin = 4;

typedef Accesses = Map<PublicID, Access>;

typedef OpenOptions = int;

typedef RWOptions = int;
const int asyncOperation = 1; // Perform the operation asynchronously
const int scheduledOperation = 2; // Schedule the operation for later

class AccessChange {
  PublicID userId = '';
  Access access = 0;

  AccessChange(this.userId, this.access);

  Map<String, dynamic> toJson() {
    return {
      'userId': userId,
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
  PrivateID userId = '';
  PublicID userPublicId = '';
  Realm realm = '';
  String url = '';
  PublicID author = '';
  Map<String, dynamic> config = {};
  StoreConfig storeConfig = const StoreConfig();

  static Vault none = Vault();

  static fromResult(Result res) {
    var s = Vault();
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
    s.realm = m['realm'] ?? '';
    return s;
  }

  /// Creates a new vault with the given identity, store configuration, and settings.
  /// The settings parameter is a map of configuration options (see Go vault.Config).
  /// Returns a tuple containing the created Bao and an error message if any.
  static Future<Vault> create(
      Realm realm, PrivateID identity, DB db, Store store,
      {Map<String, dynamic> settings = const {}}) async {
    var res = await bindings.acall(
        'bao_vault_create', [realm, identity, store.hnd, db.hnd, settings]);
    return fromResult(res);
  }

  /// Opens an existing vault with the given identity, store configuration, and author.
  /// The options parameter can be used to specify additional options for the vault.
  /// Returns a tuple containing the opened Bao and an error message if any.
  static Future<Vault> open(
      Realm realm, PrivateID identity, DB db, Store store, PublicID author,
      {Map<String, dynamic> config = const {}}) async {
    var res = await bindings.acall(
        'bao_vault_open', [realm, identity, db.hnd, store.hnd, config, author]);
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
    return res.map.map((k, v) => MapEntry<String, int>(k, v));
  }

  Future<Access> getAccess(PublicID userId) async {
    var res = await bindings.acall('bao_vault_getAccess', [hnd, userId]);
    return res.integer;
  }

  /// Waits for the specified files (by file ID) to complete pending I/O.
  /// The [fileIds] parameter is a list of file IDs to wait for.
  Future<void> waitFiles([List<int> fileIds = const []]) async {
    var res = await bindings.acall('bao_vault_waitFiles', [hnd, fileIds]);
    res.throwIfError();
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

  /// Creates a new SQL layer for the specified database.
  Future<BaoQL> baoQL(DB db) async {
    var res = await bindings.acall('bao_replica_open', [hnd, db.hnd]);
    return BaoQL(res.handle);
  }

  /// Sends a message to the specified directory.
  Future<void> send(String dir, Message message) async {
    var res = await bindings.acall('bao_mailbox_send', [hnd, dir, message]);
    res.throwIfError();
  }

  /// Receives messages from the specified directory.
  /// The [since] parameter can be used to filter messages received since a specific date.
  /// The [fromId] parameter specifies the starting message ID for pagination.
  Future<List<Message>> receive(String dir,
      {DateTime? since, int fromId = 0}) async {
    var res = await bindings.acall('bao_mailbox_receive',
        [hnd, dir, since?.millisecondsSinceEpoch ?? 0, fromId]);
    return res.list.map((x) => Message.fromJson(x)).toList();
  }

  /// Downloads the specified attachment of a message to the destination path.
  Future<void> download(
      String dir, Message message, int attachmentIndex, String dest) async {
    var res = await bindings.acall(
        'bao_mailbox_download', [hnd, dir, message, attachmentIndex, dest]);
    res.throwIfError();
  }

  Mailbox get mailbox => Mailbox(hnd);
}
