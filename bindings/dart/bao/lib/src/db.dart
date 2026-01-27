import 'dart:async';
import 'dart:io';

import 'package:bao/src/bindings.dart';
import 'package:path/path.dart' as path;



class DB {
  int hnd = 0;

  DB(int handle) {
    hnd = handle;
  }

  static Future<DB> open(String dbDriver, String dbPath, [String ddl = ""]) async {
    var res = await bindings.acall('bao_db_open', [dbDriver, dbPath, ddl]);
    res.throwIfError();
    return DB(res.handle);
  }

  static Future<DB> defaultDB() async {
    var homeDir =
        Platform.environment['HOME'] ?? Platform.environment['USERPROFILE'];
    if (homeDir == null) {
      throw UnsupportedError(
          "Default DB not supported on this platform. Use DB(String dbPath) instead.");
    }
    var dbPath = path.join(homeDir, '.config', 'bao.db');
    return DB.open("sqlite3", dbPath);
  }

  Future<void> close() async {
    var res = await bindings.acall('bao_db_close', [hnd]);
    res.throwIfError();
  }

  /// Execute a query and get a rows handle for incremental fetching.
  /// Use bao_replica_next/current/closeRows with the returned handle.
  Future<int> query(String query, Map<String, dynamic> args) async {
    var res = await bindings.acall('bao_db_query', [hnd, query, args]);
    res.throwIfError();
    return res.handle;
  }

  /// Execute a statement that does not return rows.
  Future<void> exec(String query, Map<String, dynamic> args) async {
    var res = await bindings.acall('bao_db_exec', [hnd, query, args]);
    res.throwIfError();
  }

  Future<List<dynamic>> fetchOne(String query, Map<String, dynamic> args) async {
    var res = await bindings.acall('bao_db_fetch_one', [hnd, query, args]);
    res.throwIfError();
    return res.list;
  }

  /// Fetch rows with a maximum number of rows (default 100000)
  Future<List<dynamic>> fetch(String query, Map<String, dynamic> args,
      {int maxRows = 100000}) async {
    var res = await bindings
        .acall('bao_db_fetch', [hnd, query, args, maxRows]);
    res.throwIfError();
    return res.list;
  }
}
