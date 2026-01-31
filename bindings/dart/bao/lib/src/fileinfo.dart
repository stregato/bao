import 'dart:convert';
import 'dart:typed_data';

import 'package:bao/bao.dart';

class FileInfo {
  late int id;
  late String name;
  late int size;
  late int allocatedSize;
  late DateTime modTime;
  late bool isDir;
  late int flags;
  late Uint8List attrs;
  late int keyId;
  late String storageDir;
  late String storageName;
  late PublicID authorId;
  static var none = FileInfo('', 0, DateTime.fromMillisecondsSinceEpoch(0), false, 0);

  FileInfo(this.name, this.size, this.modTime, this.isDir, this.id);

  FileInfo.fromMap(Map<String, dynamic> map) {
    id = map['id'] ?? 0;
    name = map['name'];
    size = map['size'];
    modTime = DateTime.parse(map['modTime']);
    isDir = map['isDir'];
    flags = map['flags'] ?? 0;
    allocatedSize = map['allocatedSize'] ?? 0;
    // attrs is a Go []byte; when JSON-encoded it becomes a base64 string. Decode to bytes.
    final a = map['attrs'];
    if (a == null) {
      attrs = Uint8List(0);
    } else if (a is String) {
      attrs = Uint8List.fromList(base64Decode(a));
    } else if (a is List<int>) {
      attrs = Uint8List.fromList(a);
    } else if (a is Uint8List) {
      attrs = a;
    } else {
      attrs = Uint8List(0);
    }
    keyId = map['keyId'] ?? 0;
    storageDir = map['storageDir'] ?? '';
    storageName = map['storageName'] ?? '';
    authorId = map['authorId'] ?? '';
  }
}
