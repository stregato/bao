import 'dart:convert';
import 'dart:ffi';
import 'dart:isolate';
import 'dart:typed_data';

import 'package:bao/bao.dart';
import 'package:ffi/ffi.dart';

typedef Handler = CResult Function(List<Object?> args);

final nativePointers = <Pointer>{};

Pointer<Utf8> toNativeUtf8(Object? o) {
  var p = (o as String).toNativeUtf8();
  nativePointers.add(p);
  return p;
}

Pointer<Utf8> toNativeJson(Object? o) {
  var p = jsonEncode(o).toNativeUtf8();
  nativePointers.add(p);
  return p;
}

CData toNativeData(Object? o) {
  var p = CData.alloc(o as Uint8List);
  nativePointers.add(p);
  return p.ref;
}

int toNativeInt(Object? o) {
  return o as int;
}

Object toNative(Object? o) {
  if (o is String) {
    return toNativeUtf8(o);
  } else if (o is Uint8List) {
    return toNativeData(o);
  } else if (o is int) {
    return toNativeInt(o);
  } else {
    return toNativeJson(o);
  }
}

void freeNativePointers() {
  for (var p in nativePointers) {
    malloc.free(p);
  }
  nativePointers.clear();
}

final Map<String, Handler> _handlers = <String, Handler>{
  'bao_setLogLevel': (List<Object?> args) =>
      Function.apply(libSetLogLevel, args),
  'bao_core_setHttpLog': (List<Object?> args) =>
      Function.apply(libSetHttpLog, args),
  'bao_core_getRecentLog': (List<Object?> args) =>
      Function.apply(libGetRecentLog, args),
  'bao_snapshot': (List<Object?> args) => Function.apply(libSnapshot, args),
  'bao_test': (List<Object?> args) => Function.apply(libTestFun, args),
  'bao_security_newPrivateID': (List<Object?> args) =>
      Function.apply(libNewPrivateID, args),
  'bao_security_publicID': (List<Object?> args) =>
      Function.apply(libPublicID, args),
  'bao_security_newKeyPair': (List<Object?> args) =>
      Function.apply(libNewKeyPair, args),
  'bao_security_decodeID': (List<Object?> args) =>
      Function.apply(libDecodeID, args),
  'bao_security_ecEncrypt': (List<Object?> args) =>
      Function.apply(libEcEncrypt, args),
  'bao_security_ecDecrypt': (List<Object?> args) =>
      Function.apply(libEcDecrypt, args),
  'bao_security_aesEncrypt': (List<Object?> args) =>
      Function.apply(libAesEncrypt, args),
  'bao_security_aesDecrypt': (List<Object?> args) =>
      Function.apply(libAesDecrypt, args),
  'bao_db_open': (List<Object?> args) => Function.apply(libOpenDB, args),
  'bao_db_close': (List<Object?> args) => Function.apply(libCloseDB, args),
  'bao_db_query': (List<Object?> args) => Function.apply(libDbQuery, args),
  'bao_db_exec': (List<Object?> args) => Function.apply(libDbExec, args),
  'bao_db_fetch': (List<Object?> args) => Function.apply(libDbFetch, args),
  'bao_db_fetch_one': (List<Object?> args) =>
      Function.apply(libDbFetchOne, args),
  'bao_store_open': (List<Object?> args) => Function.apply(libStoreOpen, args),
  'bao_store_close': (List<Object?> args) =>
      Function.apply(libStoreClose, args),
  'bao_store_readDir': (List<Object?> args) =>
      Function.apply(libStoreReadDir, args),
  'bao_store_stat': (List<Object?> args) => Function.apply(libStoreStat, args),
  'bao_store_delete': (List<Object?> args) =>
      Function.apply(libStoreDelete, args),
  'bao_vault_create': (List<Object?> args) =>
      Function.apply(libBaoCreate, args),
  'bao_vault_open': (List<Object?> args) => Function.apply(libBaoOpen, args),
  'bao_vault_close': (List<Object?> args) => Function.apply(libBaoClose, args),
  'bao_vault_syncAccess': (List<Object?> args) =>
      Function.apply(libBaoSyncAccess, args),
  'bao_vault_getAccesses': (List<Object?> args) =>
      Function.apply(libBaoGetAccesses, args),
  'bao_vault_getAccess': (List<Object?> args) =>
      Function.apply(libBaoGetAccess, args),
  'bao_vault_waitFiles': (List<Object?> args) =>
      Function.apply(libBaoWaitFiles, args),
  'bao_vault_sync': (List<Object?> args) => Function.apply(libBaoSync, args),
  'bao_vault_setAttribute': (List<Object?> args) =>
      Function.apply(libBaoSetAttribute, args),
  'bao_vault_getAttribute': (List<Object?> args) =>
      Function.apply(libBaoGetAttribute, args),
  'bao_vault_getAttributes': (List<Object?> args) =>
      Function.apply(libBaoGetAttributes, args),
  'bao_vault_readDir': (List<Object?> args) =>
      Function.apply(libBaoReadDir, args),
  'bao_vault_stat': (List<Object?> args) => Function.apply(libBaoStat, args),
  'bao_vault_read': (List<Object?> args) => Function.apply(libBaoRead, args),
  'bao_vault_write': (List<Object?> args) => Function.apply(libBaoWrite, args),
  'bao_vault_delete': (List<Object?> args) =>
      Function.apply(libBaoDelete, args),
  'bao_vault_allocatedSize': (List<Object?> args) =>
      Function.apply(libBaoAllocatedSize, args),
  'bao_replica_open': (List<Object?> args) =>
      Function.apply(libSqlLayerSqlLayer, args),
  'bao_replica_closeRows': (List<Object?> args) =>
      Function.apply(libSqlLayerCloseRows, args),
  'bao_replica_exec': (List<Object?> args) =>
      Function.apply(libSqlLayerExec, args),
  'bao_replica_query': (List<Object?> args) =>
      Function.apply(libSqlLayerQuery, args),
  'bao_replica_fetch': (List<Object?> args) =>
      Function.apply(libSqlLayerFetch, args),
  'bao_replica_fetchOne': (List<Object?> args) =>
      Function.apply(libSqlLayerFetchOne, args),
  'bao_replica_sync': (List<Object?> args) =>
      Function.apply(libSqlLayerSyncTables, args),
  'bao_replica_cancel': (List<Object?> args) =>
      Function.apply(libSqlLayerCancel, args),
  'bao_replica_next': (List<Object?> args) =>
      Function.apply(libSqlLayerNext, args),
  'bao_replica_current': (List<Object?> args) =>
      Function.apply(libSqlLayerCurrent, args),
  'bao_mailbox_send': (List<Object?> args) =>
      Function.apply(libMailboxSend, args),
  'bao_mailbox_receive': (List<Object?> args) =>
      Function.apply(libMailboxReceive, args),
  'bao_mailbox_download': (List<Object?> args) =>
      Function.apply(libMailboxDownload, args),
};

typedef FreeC = Void Function(Pointer<Uint8>);
typedef FreeCDart = void Function(Pointer<Uint8>);
late FreeCDart freeC;

// Global functions
late ArgsS libSetLogLevel;
late ArgsS libSetHttpLog;
late Argsi libGetRecentLog;
late Args libSnapshot;
late Args libTestFun;

// Id functions
late Args libNewPrivateID;
late Args libNewKeyPair;
late Args libPublicID;
late ArgsS libDecodeID;
late ArgsSD libEcEncrypt;
late ArgsSD libEcDecrypt;
late ArgsSDD libAesEncrypt;
late ArgsSDD libAesDecrypt;
late ArgsS libGenerateAESKey;

// DB functions
late ArgsSSS libOpenDB;
late Argsi libCloseDB;
late ArgsiSS libDbQuery;
late ArgsiSS libDbExec;
late ArgsiSSi libDbFetch;
late ArgsiSS libDbFetchOne;

// Store functions
late ArgsS libStoreOpen;
late Argsi libStoreClose;
late ArgsiSS libStoreReadDir;
late ArgsiS libStoreStat;
late ArgsiS libStoreDelete;

// Bao functions
late Argssiis libBaoCreate;
late ArgsiSSii libBaoOpen;
late Argsi libBaoClose;
late Argsiis libBaoSyncAccess;
late Argsi libBaoGetAccesses;
late Argsi libBaoGetAccess;
late ArgsiS libBaoWaitFiles;
late Argsi libBaoSync;
late Argsiiss libBaoSetAttribute;
late ArgsiSS libBaoGetAttribute;
late ArgsiS libBaoGetAttributes;
late ArgsiSiii libBaoReadDir;
late ArgsiS libBaoStat;
late ArgsiSSi libBaoRead;
late ArgsiSSDi libBaoWrite;
late ArgsiSi libBaoDelete;
late Argsi libBaoAllocatedSize;

// SQL Layer functions
late ArgsiI libSqlLayerSqlLayer;
late ArgsiSS libSqlLayerExec;
late ArgsiSS libSqlLayerQuery;
late ArgsiSSi libSqlLayerFetch;
late ArgsiSS libSqlLayerFetchOne;
late Argsi libSqlLayerSyncTables;
late Argsi libSqlLayerCancel;
late Argsi libSqlLayerNext;
late Argsi libSqlLayerCurrent;
late Argsi libSqlLayerCloseRows;

// Mailbox functions
late ArgsiSS libMailboxSend;
late ArgsiSii libMailboxReceive;
late ArgsiSSiS libMailboxDownload;

void loadFunctions(DynamicLibrary lib) {
  freeC = lib.lookupFunction<FreeC, FreeCDart>('free');
  libSetLogLevel = lib.lookupFunction<ArgsS, ArgsS>('bao_setLogLevel');
  libSetHttpLog = lib.lookupFunction<ArgsS, ArgsS>('bao_core_setHttpLog');
  libGetRecentLog = lib.lookupFunction<ArgsI, Argsi>('bao_core_getRecentLog');
  libSnapshot = lib.lookupFunction<Args, Args>('bao_snapshot');
  libTestFun = lib.lookupFunction<Args, Args>('bao_test');

  libNewPrivateID =
      lib.lookupFunction<Args, Args>('bao_security_newPrivateID');
  libNewKeyPair =
      lib.lookupFunction<Args, Args>('bao_security_newKeyPair');
  libPublicID = lib.lookupFunction<Args, Args>('bao_security_publicID');
  libDecodeID = lib.lookupFunction<ArgsS, ArgsS>('bao_security_decodeID');
  libEcEncrypt = lib.lookupFunction<ArgsSD, ArgsSD>('bao_security_ecEncrypt');
  libEcDecrypt = lib.lookupFunction<ArgsSD, ArgsSD>('bao_security_ecDecrypt');
  libAesEncrypt =
      lib.lookupFunction<ArgsSDD, ArgsSDD>('bao_security_aesEncrypt');
  libAesDecrypt =
      lib.lookupFunction<ArgsSDD, ArgsSDD>('bao_security_aesDecrypt');

  libOpenDB = lib.lookupFunction<ArgsSSS, ArgsSSS>("bao_db_open");
  libCloseDB = lib.lookupFunction<ArgsI, Argsi>("bao_db_close");
  libDbQuery = lib.lookupFunction<ArgsISS, ArgsiSS>("bao_db_query");
  libDbExec = lib.lookupFunction<ArgsISS, ArgsiSS>("bao_db_exec");
  libDbFetch = lib.lookupFunction<ArgsISSIi, ArgsiSSi>("bao_db_fetch");
  libDbFetchOne =
      lib.lookupFunction<ArgsISS, ArgsiSS>("bao_db_fetch_one");

  libStoreOpen = lib.lookupFunction<ArgsS, ArgsS>('bao_store_open');
  libStoreClose = lib.lookupFunction<ArgsI, Argsi>('bao_store_close');
  libStoreReadDir = lib.lookupFunction<ArgsISS, ArgsiSS>('bao_store_readDir');
  libStoreStat = lib.lookupFunction<ArgsIS, ArgsiS>('bao_store_stat');
  libStoreDelete = lib.lookupFunction<ArgsIS, ArgsiS>('bao_store_delete');

  libBaoCreate =
      lib.lookupFunction<ArgsSSIIS, Argssiis>("bao_vault_create");
  libBaoOpen =
      lib.lookupFunction<ArgsSSSII, ArgsiSSii>("bao_vault_open");
  libBaoClose = lib.lookupFunction<ArgsI, Argsi>("bao_vault_close");
  libBaoSyncAccess =
      lib.lookupFunction<ArgsIiS, Argsiis>("bao_vault_syncAccess");
  libBaoGetAccesses =
      lib.lookupFunction<ArgsI, Argsi>("bao_vault_getAccesses");
  libBaoGetAccess = lib.lookupFunction<ArgsI, Argsi>("bao_vault_getAccess");
  libBaoWaitFiles =
      lib.lookupFunction<ArgsIS, ArgsiS>("bao_vault_waitFiles");
  libBaoSync = lib.lookupFunction<ArgsI, Argsi>("bao_vault_sync");
  libBaoSetAttribute =
      lib.lookupFunction<ArgsIiSS, Argsiiss>("bao_vault_setAttribute");
  libBaoGetAttribute =
      lib.lookupFunction<ArgsISS, ArgsiSS>("bao_vault_getAttribute");
  libBaoGetAttributes =
      lib.lookupFunction<ArgsIS, ArgsiS>("bao_vault_getAttributes");
  libBaoReadDir =
      lib.lookupFunction<ArgsISIIi, ArgsiSiii>("bao_vault_readDir");
  libBaoStat = lib.lookupFunction<ArgsIS, ArgsiS>("bao_vault_stat");
  libBaoRead = lib.lookupFunction<ArgsISSI, ArgsiSSi>("bao_vault_read");
    libBaoWrite = lib.lookupFunction<ArgsISSDI, ArgsiSSDi>("bao_vault_write");
    libBaoDelete = lib.lookupFunction<ArgsISi, ArgsiSi>("bao_vault_delete");
  libBaoAllocatedSize =
      lib.lookupFunction<ArgsI, Argsi>("bao_vault_allocatedSize");

    libSqlLayerSqlLayer = lib.lookupFunction<ArgsIi, ArgsiI>('bao_replica_open');
  libSqlLayerExec = lib.lookupFunction<ArgsISS, ArgsiSS>('bao_replica_exec');
  libSqlLayerQuery = lib.lookupFunction<ArgsISS, ArgsiSS>('bao_replica_query');
  libSqlLayerFetch =
      lib.lookupFunction<ArgsISSIi, ArgsiSSi>('bao_replica_fetch');
  libSqlLayerFetchOne =
      lib.lookupFunction<ArgsISS, ArgsiSS>('bao_replica_fetchOne');
  libSqlLayerSyncTables =
      lib.lookupFunction<ArgsI, Argsi>('bao_replica_sync');
  libSqlLayerCancel = lib.lookupFunction<ArgsI, Argsi>('bao_replica_cancel');
  libSqlLayerNext = lib.lookupFunction<ArgsI, Argsi>('bao_replica_next');
  libSqlLayerCurrent =
      lib.lookupFunction<ArgsI, Argsi>('bao_replica_current');
  libSqlLayerCloseRows =
      lib.lookupFunction<ArgsI, Argsi>('bao_replica_closeRows');

  libMailboxSend =
      lib.lookupFunction<ArgsISS, ArgsiSS>('bao_mailbox_send');
  libMailboxReceive =
      lib.lookupFunction<ArgsISII, ArgsiSii>('bao_mailbox_receive');
  libMailboxDownload =
      lib.lookupFunction<ArgsISSIiS, ArgsiSSiS>('bao_mailbox_download');
}

// ---- Worker entry: opens lib once, caches lookups once
void _worker(SendPort mainSendPort) async {
  final lib = loadBaoLibrary();
  loadFunctions(lib);

  final inbox = ReceivePort();
  mainSendPort.send(inbox.sendPort); // only the SendPort goes back

  await for (final args in inbox) {
    String name = args[0] as String;
    final output = args[1] as SendPort;
    final handler = _handlers[name];

    if (handler == null) {
      output.send(Result(0, Uint8List(0),
          Err(msg: "No handler found for $name. This is a code error.")));
      continue;
    }

    try {
      var nativeArgs = args.sublist(2).map(toNative).toList();
      output.send(handler(nativeArgs).resolve());
    } catch (e) {
      output.send(Result(0, Uint8List(0), Err(msg: e.toString())));
    } finally {
      freeNativePointers();
    }
  }
}

class Bindings {
  final inputs = <SendPort>[];

  Future<void> start([int size = 4]) async {
    for (var i = 0; i < size; i++) {
      var rp = ReceivePort();
      await Isolate.spawn(_worker, rp.sendPort);
      inputs.add(await rp.first as SendPort);
    }
  }

// Round-robin dispatch
  int next = -1;

  Future<Result> acall(String name, List<Object?> args) async {
    next = (next + 1) % inputs.length;
    var output = ReceivePort();
    inputs[next].send([name, output.sendPort, ...args]);
    var res = await output.first;
    output.close();
    return res as Result;
  }

  Result call(String name, List<Object?> args) {
    final handler = _handlers[name];

    if (handler != null) {
      var nativeArgs = args.map(toNative).toList();
      var res = handler(nativeArgs).resolve();
      freeNativePointers();
      return res;
    }

    throw ArgumentError("No handler found for $name");
  }
}

final bindings = Bindings();
