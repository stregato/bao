import 'dart:convert';
import 'dart:ffi';
import 'dart:io';

import 'package:bao/src/bindings.dart';
import 'package:ffi/ffi.dart';
import 'package:flutter/foundation.dart';

class Err {
  String msg;
  String file;
  int line;
  Err? cause;

  Err({this.msg = "", this.file = "", this.line = 0, this.cause});

  static String shortWrite = "short write";
  static String invalidWrite = "invalid write result";
  static String shortBuffer = "short buffer";
  static String eof = "EOF";
  static String unexpectedEof = "unexpected EOF";
  static String noProgress = "multiple Read calls return no data or error";
  static String invalid = "invalid argument";
  static String permission = "permission denied";
  static String exist = "file already exists";
  static String notExist = "file does not exist";
  static String closed = "file already closed";
  static String accessRevoked = "access revoked";

  static fromJson(Map<String, dynamic> json) {
    return Err(
      msg: json['msg'] ?? "",
      file: json['file'] ?? "",
      line: json['line'] ?? 0,
      cause: json['cause'] != null && json['cause'].isNotEmpty
          ? Err.fromJson(json['cause'])
          : null,
    );
  }

  bool get isEmpty => msg.isEmpty && file.isEmpty && line == 0 && cause == null;
  isNotEmpty() => !isEmpty;

  isType(String type) {
    return msg.contains(type) || (cause?.isType(type) ?? false);
  }

  @override
  String toString() {
    return '$file[$line]: $msg\n\t$cause';
  }
}

class Result {
  int hnd;
  Uint8List value;
  Err? error;

  Result(this.hnd, this.value, this.error);

  void throwIfError() {
    if (error != null) {
      throw error!;
    }
  }

  int get handle {
    throwIfError();
    return hnd;
  }

  String get string {
    throwIfError();
    return jsonDecode(String.fromCharCodes(value)) as String;
  }

  int get integer {
    throwIfError();
    return jsonDecode(String.fromCharCodes(value)) as int;
  }

  bool get boolean {
    throwIfError();
    return jsonDecode(String.fromCharCodes(value)) as bool;
  }

  Map<String, dynamic> get map {
    throwIfError();
    if (value.isEmpty) {
      return <String, dynamic>{};
    }

    return jsonDecode(String.fromCharCodes(value)) as Map<String, dynamic>;
  }

  List<dynamic> get list {
    throwIfError();
    if (value.isEmpty) {
      return [];
    }

    var ls = jsonDecode(String.fromCharCodes(value));
    return ls == null ? [] : ls as List<dynamic>;
  }

  Uint8List get data {
    throwIfError();
    return value;
  }
}

sealed class CResult extends Struct {
  external Pointer<Uint8> ptr;
  @Uint64()
  external int len;
  @Int64()
  external int hnd;
  external Pointer<Utf8> err;

  Result resolve() {
    Uint8List data = Uint8List(0);
    if (ptr.address != 0 && len > 0) {
      data = Uint8List.fromList(ptr.asTypedList(len));
      freeC(ptr);
      ptr = Pointer<Uint8>.fromAddress(0);
      len = 0;
    }
    if (err.address != 0) {
      var payload = err.toDartString();
      freeC(err.cast());
      err = Pointer<Utf8>.fromAddress(0);
      if (kDebugMode) {
        print('Last log lines:');
        for (var line in getRecentLog(100)) {
          print(line);
        }
        print('CResult error payload: $payload');
      }
      try {
        var asJson = jsonDecode(payload);
        return Result(hnd, Uint8List(0), Err.fromJson(asJson));
      } catch (e) {
        return Result(hnd, Uint8List(0), Err(msg: payload));
      }
    }
    return Result(hnd, data, null);
  }
}

sealed class CData extends Struct {
  external Pointer<Uint8> ptr;
  @Uint64()
  external int len;

  // Deprecated: prefer alloc which returns a pointer you can free.
  // Keeping for compatibility in case it's referenced elsewhere.
  static CData fromUint8List(Uint8List data) {
    final p = alloc(data);
    // Returning ref leaks the struct because caller can't free it.
    // Avoid using this in new code.
    return p.ref;
  }

  // Allocate a CData struct and buffer, returning a pointer suitable for
  // native signatures expecting *C.Data.
  static Pointer<CData> alloc(Uint8List data) {
    final p = calloc<CData>();
    final byteArray = calloc<Uint8>(data.length);
    for (int i = 0; i < data.length; i++) {
      byteArray[i] = data[i];
    }
    p.ref.ptr = byteArray;
    p.ref.len = data.length;
    return p;
  }

  // Free both the inner buffer and the struct itself.
  static void freeAllocated(Pointer<CData> p) {
    if (p.address != 0) {
      if (p.ref.ptr.address != 0) {
        calloc.free(p.ref.ptr);
      }
      calloc.free(p);
    }
  }
}

class CException implements Exception {
  String msg;
  CException(this.msg);

  @override
  String toString() {
    return msg;
  }
}

typedef Args = CResult Function();
typedef ArgsS = CResult Function(Pointer<Utf8>);

typedef ArgsI = CResult Function(Int64);
typedef Argsi = CResult Function(int);

typedef ArgsII = CResult Function(Int64, Int64);
typedef Argsii = CResult Function(int, int);

typedef ArgsSS = CResult Function(Pointer<Utf8>, Pointer<Utf8>);
// Three C strings
typedef ArgsSSS = CResult Function(Pointer<Utf8>, Pointer<Utf8>, Pointer<Utf8>);

// Two strings, two 64-bit ints, one string
typedef ArgsSSIIS = CResult Function(
  Pointer<Utf8>, Pointer<Utf8>, Int64, Int64, Pointer<Utf8>);
// Two strings, two 64-bit ints, two strings
typedef ArgsSSIISS = CResult Function(Pointer<Utf8>, Pointer<Utf8>, Int64,
  Int64, Pointer<Utf8>, Pointer<Utf8>);

typedef ArgsIS = CResult Function(Int64, Pointer<Utf8>);
typedef ArgsiS = CResult Function(int, Pointer<Utf8>);

typedef ArgsISS = CResult Function(Int64, Pointer<Utf8>, Pointer<Utf8>);
typedef ArgsiSS = CResult Function(int, Pointer<Utf8>, Pointer<Utf8>);

typedef ArgsIISS = CResult Function(Int64, Int64, Pointer<Utf8>, Pointer<Utf8>);
typedef ArgsiiSS = CResult Function(int, int, Pointer<Utf8>, Pointer<Utf8>);

// 64-bit int, 32-bit int, two strings
typedef ArgsIiSS = CResult Function(Int64, Int32, Pointer<Utf8>, Pointer<Utf8>);

// (char*, *Data)
typedef ArgsSD = CResult Function(Pointer<Utf8>, CData);
// (char*, *Data, *Data)
typedef ArgsSDD = CResult Function(Pointer<Utf8>, CData, CData);

typedef ArgsISI = CResult Function(Int64, Pointer<Utf8>, Int64);
typedef ArgsiSi = CResult Function(int, Pointer<Utf8>, int);

typedef ArgsISi = CResult Function(Int64, Pointer<Utf8>, Int32);

typedef ArgsIII = CResult Function(Int64, Int64, Int64);
typedef Argsiii = CResult Function(int, int, int);

typedef ArgsISIS = CResult Function(Int64, Pointer<Utf8>, Int64, Pointer<Utf8>);
typedef ArgsiSiS = CResult Function(int, Pointer<Utf8>, int, Pointer<Utf8>);

typedef ArgsIiS = CResult Function(Int64, Int32, Pointer<Utf8>);
typedef Argsiis = CResult Function(int, int, Pointer<Utf8>);

// Correct native signature when the 3rd arg is C.int (32-bit)
typedef ArgsISiS = CResult Function(Int64, Pointer<Utf8>, Int32, Pointer<Utf8>);
// Dart-side remains ArgsiSiS (int maps to C.int)

typedef ArgsISSS = CResult Function(
    Int64, Pointer<Utf8>, Pointer<Utf8>, Pointer<Utf8>);
typedef ArgsiSSS = CResult Function(
    int, Pointer<Utf8>, Pointer<Utf8>, Pointer<Utf8>);
typedef Argssiis = CResult Function(
    Pointer<Utf8>, Pointer<Utf8>, int, int, Pointer<Utf8>);
typedef Argssiiss = CResult Function(Pointer<Utf8>, Pointer<Utf8>, int, int,
    Pointer<Utf8>, Pointer<Utf8>);

typedef ArgsSIIS = CResult Function(
    Pointer<Utf8>, Int64, Int64, Pointer<Utf8>);
typedef ArgsSiis = CResult Function(
    Pointer<Utf8>, int, int, Pointer<Utf8>);

typedef Argsiiss = CResult Function(int, int, Pointer<Utf8>, Pointer<Utf8>);
typedef ArgsSIISS = CResult Function(
    Pointer<Utf8>, Int64, Int64, Pointer<Utf8>, Pointer<Utf8>);
typedef ArgsSiiss = CResult Function(
    Pointer<Utf8>, int, int, Pointer<Utf8>, Pointer<Utf8>);

typedef ArgsISSI = CResult Function(Int64, Pointer<Utf8>, Pointer<Utf8>, Int64);
typedef ArgsiSSi = CResult Function(int, Pointer<Utf8>, Pointer<Utf8>, int);

typedef ArgsISII = CResult Function(Int64, Pointer<Utf8>, Int64, Int64);
typedef ArgsiSii = CResult Function(int, Pointer<Utf8>, int, int);

typedef ArgsISSIS = CResult Function(
    Int64, Pointer<Utf8>, Pointer<Utf8>, Int64, Pointer<Utf8>);
typedef ArgsiSSiS = CResult Function(
    int, Pointer<Utf8>, Pointer<Utf8>, int, Pointer<Utf8>);

typedef ArgsISSDI = CResult Function(
    Int64, Pointer<Utf8>, Pointer<Utf8>, CData, Int64);
typedef ArgsiSSDi = CResult Function(
    int, Pointer<Utf8>, Pointer<Utf8>, CData, int);

typedef ArgsISISI = CResult Function(
    Int64, Pointer<Utf8>, Int64, Pointer<Utf8>, Int64);
typedef ArgsiSiSi = CResult Function(
    int, Pointer<Utf8>, int, Pointer<Utf8>, int);

typedef ArgsISSSI = CResult Function(
    Int64, Pointer<Utf8>, Pointer<Utf8>, Pointer<Utf8>, Int64);
typedef ArgsiSSSi = CResult Function(
    int, Pointer<Utf8>, Pointer<Utf8>, Pointer<Utf8>, int);

typedef ArgsISIII = CResult Function(Int64, Pointer<Utf8>, Int64, Int64, Int64);
typedef ArgsiSiii = CResult Function(int, Pointer<Utf8>, int, int, int);

typedef ArgsISSDSI = CResult Function(
    Int64, Pointer<Utf8>, Pointer<Utf8>, Pointer<Utf8>, CData, Int64);
typedef ArgsiSSDSi = CResult Function(
    int, Pointer<Utf8>, Pointer<Utf8>, Pointer<Utf8>, CData, int);

typedef ArgsISDSSI = CResult Function(
    Int64, Pointer<Utf8>, Pointer<Utf8>, CData, Pointer<Utf8>, Int64);

// Native signatures with C.int parameters
typedef ArgsISIIi = CResult Function(
    Int64, Pointer<Utf8>, Int64, Int64, Int32); // last param C.int
typedef ArgsISSIi = CResult Function(
    Int64, Pointer<Utf8>, Pointer<Utf8>, Int32); // last param C.int
typedef ArgsISSIiS = CResult Function(Int64, Pointer<Utf8>, Pointer<Utf8>,
    Int32, Pointer<Utf8>); // includes dest string

// 64-bit int, 32-bit int
typedef ArgsIi = CResult Function(Int64, Int32);
typedef ArgsiI = CResult Function(int, int);

//late DynamicLibrary baoLibrary;

DynamicLibrary loadBaoLibrary() {
  var arch = _getArch();

  // Special handling for iOS since it doesn't use DynamicLibrary.open
  if (Platform.isIOS) {
    try {
      var lib = DynamicLibrary.process();
      return lib;
    } catch (e) {
      // skip
    }
  } else {
    for (var file in _libraryPaths[arch]!) {
      try {
        var libraryPath = File(file);
        // Load the dynamic library from the library path
        var lib = DynamicLibrary.open(libraryPath.path);
        return lib;
      } catch (e) {
        //skip
      }
    }
  }

  throw Err(msg: 'Failed to load bao library for $arch');
}

Future<void> initBaoLibrary() async {
  var lib = loadBaoLibrary();
  freeC = lib.lookupFunction<FreeC, FreeCDart>('free');

  loadFunctions(lib);
  await bindings.start();
}

var _libraryPaths = {
  'linux-amd64': ['lib/linux/libbao.so', 'libbao.so'],
  'linux-arm64': ['lib/linux/libbao.so', 'libbao.so'],
  'macos-amd64': ['libbao_amd64.dylib'],
  'macos-arm64': [
    'libbao_arm64.dylib',
    '../../../build/darwin/libbao_arm64.dylib'
  ],
  'windows-amd64': ['baod.dll'],
};

String _getArch() {
  // Get the operating system and architecture to construct the folder name
  var os = Platform.operatingSystem; // linux, macos, windows, android, ios
  final arch = Platform.version.toLowerCase();

  String archFolder;

  if (arch.contains('amd64') || arch.contains('x64')) {
    archFolder = 'amd64';
  } else if (arch.contains('arm64')) {
    archFolder = 'arm64';
  } else {
    throw Exception('Unsupported architecture: $arch');
  }

  // Compose the folder name using the operating system and architecture
  return '$os-$archFolder';
}


void setLogLevel(String level) {
  bindings.call('bao_setLogLevel', [level]).throwIfError();
}

List<String> getRecentLog(int n) {
  var r = bindings.call('bao_core_getRecentLog', [n]);
  return (r.list).map((e) => e as String).toList();
}

void setHttpLog(String url) {
  bindings.call('bao_core_setHttpLog', [url]).throwIfError();
}

String snapshot() {
  var r = bindings.call('bao_snapshot', []);
  return r.string;
}

void baoTest() {
  bindings.call('bao_test', []).throwIfError();
}
