import 'dart:convert';

import 'package:bao/src/bindings.dart';

class Rows {
  int hnd;
  Rows(this.hnd);

  /// Advances to the next row in the result set.
  /// Returns `true` if there is a next row, `false` otherwise.
  bool next() {
    return bindings.call('bao_replica_next', [hnd]).boolean;
  }

  /// Returns the current row as a list of dynamic values.
  /// If there is no current row or an error occurs, returns an empty list.
  List<dynamic> current() {
    return bindings.call('bao_replica_current', [hnd]).list;
  }

  /// Closes the rows handle, releasing any associated resources.
  /// No need to call this method if fetch is used, as it will automatically close the handle.
  void close() {
    bindings.call('bao_replica_closeRows', [hnd]);
  }
}

class BaoQL {
  int hnd;
  BaoQL(this.hnd);
  static BaoQL none = BaoQL(0);

  void close() async {
    bindings.call('bao_replica_cancel', [hnd]).throwIfError();
  }

  // Executes a SQL query with the provided arguments.
  /// Returns an error if the execution fails.
  /// The query should be a valid SQL statement, and args should be a map of parameters.
  Future<void> exec(String query, Map<String, dynamic> args) async {
    await bindings
        .acall('bao_replica_exec', [hnd, query, jsonEncode(args)]);
  }

  // Executes a SQL query and returns the result as a Rows object.
  /// Returns a tuple containing the Rows object and an error message if any.
  Future<Rows> query(String query, Map<String, dynamic> args) async {
    var res = await bindings.acall(
        'bao_replica_query', [hnd, query, jsonEncode(args)]);
    return Rows(res.handle);
  }

  /// Executes a SQL query and returns the result as a list of dynamic values.
  /// Returns a tuple containing the list of dynamic values and an error message if any.
  /// The query should be a valid SQL statement, and args should be a map of parameters.
  Future<List<dynamic>> fetch(String query, Map<String, dynamic> args,
      {int maxRows = 100000}) async {
    var res = await bindings.acall(
        'bao_replica_fetch', [hnd, query, jsonEncode(args), maxRows]);
    return res.list;
  }

  /// Executes a SQL query and returns the result the first row as a list of dynamic values.
  /// Returns a tuple containing the list of dynamic values and an error message if any.
  List<dynamic> fetchOne(String query, Map<String, dynamic> args) {
    var res = bindings.call(
        'bao_replica_fetchOne', [hnd, query, jsonEncode(args)]);
    return res.list;
  }

  /// Synchronizes the SQL layer tables with the underlying storage.
  /// Returns an error if the synchronization fails.
  Future<int> syncTables() async {
    var res = await bindings.acall('bao_replica_sync', [hnd]);
    return res.integer;
  }

  /// Cancels any ongoing operations in the SQL layer.
  /// Returns an error if the cancellation fails.
  void cancel() {
    bindings.call('bao_replica_cancel', [hnd]).throwIfError();
  }
}
