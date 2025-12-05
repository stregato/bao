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
  'bao_setHttpLog': (List<Object?> args) =>
      Function.apply(libSetHttpLog, args),
  'bao_getRecentLog': (List<Object?> args) =>
      Function.apply(libGetRecentLog, args),
  'bao_snapshot': (List<Object?> args) => Function.apply(libSnapshot, args),
  'bao_test': (List<Object?> args) => Function.apply(libTestFun, args),
  'bao_newPrivateID': (List<Object?> args) =>
      Function.apply(libNewPrivateID, args),
  'bao_publicID': (List<Object?> args) => Function.apply(libPublicID, args),
  'bao_decodeID': (List<Object?> args) => Function.apply(libDecodeID, args),
  'bao_ecEncrypt': (List<Object?> args) => Function.apply(libEcEncrypt, args),
  'bao_ecDecrypt': (List<Object?> args) => Function.apply(libEcDecrypt, args),
  'bao_aesEncrypt': (List<Object?> args) =>
      Function.apply(libAesEncrypt, args),
  'bao_aesDecrypt': (List<Object?> args) =>
      Function.apply(libAesDecrypt, args),
  'bao_openDB': (List<Object?> args) => Function.apply(libOpenDB, args),
  'bao_closeDB': (List<Object?> args) => Function.apply(libCloseDB, args),
  'bao_dbQuery': (List<Object?> args) => Function.apply(libDbQuery, args),
  'bao_dbExec': (List<Object?> args) => Function.apply(libDbExec, args),
  'bao_dbFetch': (List<Object?> args) => Function.apply(libDbFetch, args),
  'bao_dbFetchOne': (List<Object?> args) =>
      Function.apply(libDbFetchOne, args),
  'bao_create': (List<Object?> args) => Function.apply(libBaoCreate, args),
  'bao_open': (List<Object?> args) => Function.apply(libBaoOpen, args),
  'bao_close': (List<Object?> args) => Function.apply(libBaoClose, args),
  'bao_syncAccess': (List<Object?> args) =>
      Function.apply(libBaoSyncAccess, args),
  'bao_getAccess': (List<Object?> args) =>
      Function.apply(libBaoGetAccess, args),
  'bao_getGroups': (List<Object?> args) =>
      Function.apply(libBaoGetGroups, args),
  'bao_waitFiles': (List<Object?> args) =>
      Function.apply(libBaoWaitFiles, args),
  'bao_listGroups': (List<Object?> args) =>
      Function.apply(libBaoListGroups, args),
  'bao_sync': (List<Object?> args) => Function.apply(libBaoSync, args),
  'bao_setAttribute': (List<Object?> args) =>
      Function.apply(libBaoSetAttribute, args),
  'bao_getAttribute': (List<Object?> args) =>
      Function.apply(libBaoGetAttribute, args),
  'bao_getAttributes': (List<Object?> args) =>
      Function.apply(libBaoGetAttributes, args),
  'bao_readDir': (List<Object?> args) =>
      Function.apply(libBaoReadDir, args),
  'bao_stat': (List<Object?> args) => Function.apply(libBaoStat, args),
  'bao_read': (List<Object?> args) => Function.apply(libBaoRead, args),
  'bao_write': (List<Object?> args) => Function.apply(libBaoWrite, args),
  'bao_delete': (List<Object?> args) => Function.apply(libBaoDelete, args),
  'baoql_layer': (List<Object?> args) =>
      Function.apply(libSqlLayerSqlLayer, args),
  'baoql_closeRows': (List<Object?> args) =>
      Function.apply(libSqlLayerCloseRows, args),
  'baoql_exec': (List<Object?> args) =>
      Function.apply(libSqlLayerExec, args),
  'baoql_query': (List<Object?> args) =>
      Function.apply(libSqlLayerQuery, args),
  'baoql_fetch': (List<Object?> args) =>
      Function.apply(libSqlLayerFetch, args),
  'baoql_fetchOne': (List<Object?> args) =>
      Function.apply(libSqlLayerFetchOne, args),
  'baoql_sync_tables': (List<Object?> args) =>
      Function.apply(libSqlLayerSyncTables, args),
  'baoql_cancel': (List<Object?> args) =>
      Function.apply(libSqlLayerCancel, args),
  'mailbox_send': (List<Object?> args) => Function.apply(libMailboxSend, args),
  'mailbox_receive': (List<Object?> args) =>
      Function.apply(libMailboxReceive, args),
  'mailbox_download': (List<Object?> args) =>
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
late ArgsS libPublicID;
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

// Bao functions
late ArgsiSSS libBaoCreate;
late ArgsiSSS libBaoOpen;
late Argsi libBaoClose;
late Argsiis libBaoSyncAccess;
late ArgsiS libBaoGetAccess;
late ArgsiS libBaoGetGroups;
late ArgsiS libBaoWaitFiles;
late Argsi libBaoListGroups;
late ArgsiS libBaoSync;
late ArgsiiSS libBaoSetAttribute;
late ArgsiSS libBaoGetAttribute;
late ArgsiS libBaoGetAttributes;
late ArgsiSiii libBaoReadDir;
late ArgsiS libBaoStat;
late ArgsiSSi libBaoRead;
late ArgsiSSDSi libBaoWrite;
late ArgsiSi libBaoDelete;

// SQL Layer functions
late ArgsiSi libSqlLayerSqlLayer;
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
late ArgsiSSS libMailboxSend;
late ArgsiSii libMailboxReceive;
late ArgsiSSiS libMailboxDownload;

void loadFunctions(DynamicLibrary lib) {
  freeC = lib.lookupFunction<FreeC, FreeCDart>('free');
  libSetLogLevel = lib.lookupFunction<ArgsS, ArgsS>('bao_setLogLevel');
  libSetHttpLog = lib.lookupFunction<ArgsS, ArgsS>('bao_setHttpLog');
  libGetRecentLog = lib.lookupFunction<ArgsI, Argsi>('bao_getRecentLog');
  libSnapshot = lib.lookupFunction<Args, Args>('bao_snapshot');
  libTestFun = lib.lookupFunction<Args, Args>('bao_test');

  libNewPrivateID = lib.lookupFunction<Args, Args>('bao_newPrivateID');
  libPublicID = lib.lookupFunction<ArgsS, ArgsS>('bao_publicID');
  libDecodeID = lib.lookupFunction<ArgsS, ArgsS>('bao_decodeID');
  libEcEncrypt = lib.lookupFunction<ArgsSD, ArgsSD>('bao_ecEncrypt');
  libEcDecrypt = lib.lookupFunction<ArgsSD, ArgsSD>('bao_ecDecrypt');
  libAesEncrypt = lib.lookupFunction<ArgsSDD, ArgsSDD>('bao_aesEncrypt');
  libAesDecrypt = lib.lookupFunction<ArgsSDD, ArgsSDD>('bao_aesDecrypt');

  libOpenDB = lib.lookupFunction<ArgsSSS, ArgsSSS>("bao_openDB");
  libCloseDB = lib.lookupFunction<ArgsI, Argsi>("bao_closeDB");
  libDbQuery = lib.lookupFunction<ArgsISS, ArgsiSS>("bao_dbQuery");
  libDbExec = lib.lookupFunction<ArgsISS, ArgsiSS>("bao_dbExec");
  libDbFetch = lib.lookupFunction<ArgsISSI, ArgsiSSi>("bao_dbFetch");
  libDbFetchOne = lib.lookupFunction<ArgsISS, ArgsiSS>("bao_dbFetchOne");
  libBaoCreate = lib.lookupFunction<ArgsISSS, ArgsiSSS>("bao_create");
  libBaoOpen = lib.lookupFunction<ArgsISSS, ArgsiSSS>("bao_open");
  libBaoClose = lib.lookupFunction<ArgsI, Argsi>("bao_close");
  libBaoSyncAccess = lib.lookupFunction<ArgsIiS, Argsiis>("bao_syncAccess");
  libBaoGetAccess = lib.lookupFunction<ArgsIS, ArgsiS>("bao_getAccess");
  libBaoGetGroups = lib.lookupFunction<ArgsIS, ArgsiS>("bao_getGroups");
  libBaoWaitFiles = lib.lookupFunction<ArgsIS, ArgsiS>("bao_waitFiles");
  libBaoListGroups = lib.lookupFunction<ArgsI, Argsi>("bao_listGroups");
  libBaoSync = lib.lookupFunction<ArgsIS, ArgsiS>("bao_sync");
  libBaoSetAttribute =
      lib.lookupFunction<ArgsIISS, ArgsiiSS>("bao_setAttribute");
  libBaoGetAttribute =
      lib.lookupFunction<ArgsISS, ArgsiSS>("bao_getAttribute");
  libBaoGetAttributes =
      lib.lookupFunction<ArgsIS, ArgsiS>("bao_getAttributes");
  libBaoReadDir = lib.lookupFunction<ArgsISIII, ArgsiSiii>("bao_readDir");
  libBaoStat = lib.lookupFunction<ArgsIS, ArgsiS>("bao_stat");
  libBaoRead = lib.lookupFunction<ArgsISSI, ArgsiSSi>("bao_read");
  libBaoWrite = lib.lookupFunction<ArgsISSDSI, ArgsiSSDSi>("bao_write");
  libBaoDelete = lib.lookupFunction<ArgsISI, ArgsiSi>("bao_delete");

  libSqlLayerSqlLayer =
      lib.lookupFunction<ArgsISI, ArgsiSi>('baoql_layer');
  libSqlLayerExec = lib.lookupFunction<ArgsISS, ArgsiSS>('baoql_exec');
  libSqlLayerQuery = lib.lookupFunction<ArgsISS, ArgsiSS>('baoql_query');
  libSqlLayerFetch = lib.lookupFunction<ArgsISSI, ArgsiSSi>('baoql_fetch');
  libSqlLayerFetchOne =
      lib.lookupFunction<ArgsISS, ArgsiSS>('baoql_fetchOne');
  libSqlLayerSyncTables =
      lib.lookupFunction<ArgsI, Argsi>('baoql_sync_tables');
  libSqlLayerCancel = lib.lookupFunction<ArgsI, Argsi>('baoql_cancel');
  libSqlLayerNext = lib.lookupFunction<ArgsI, Argsi>('baoql_next');
  libSqlLayerCurrent = lib.lookupFunction<ArgsI, Argsi>('baoql_current');
  libSqlLayerCloseRows = lib.lookupFunction<ArgsI, Argsi>('baoql_closeRows');

  libMailboxSend = lib.lookupFunction<ArgsISSS, ArgsiSSS>('mailbox_send');
  libMailboxReceive = lib.lookupFunction<ArgsISII, ArgsiSii>('mailbox_receive');
  libMailboxDownload =
      lib.lookupFunction<ArgsISSIiS, ArgsiSSiS>('mailbox_download');
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
