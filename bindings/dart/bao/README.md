bao is a Dart binding for the Bao library. It enables encrypted data storage and exchange. Similar to traditional systems, Bao offers controlled access with a security model inspired by Unix permissions.
Unlike traditional storage, the access is based on cryptographic features, enabling distributed control.

## Features
Bao offers convenient  

It supports Android, iOS, macOS, Linux and Windows on the common architectures for each OS.

## Getting started
Run _flutter pub get bao_ or _dart pub get bao_ from your terminal.
Alternatevely add _bao_ dependency in your pubspec.yaml file.

The package only contains the dart source code. You still need to download the binary libraries (depending on the target architecture) from http://github.com/stregato/bao and copy to the subfolders for your architecture, i.e. _linux_, _ios_, _macos_, _android_, _windows_.

The script http://github.com/stregato/bao/bindings/dart/install.sh automatically download the libraries and place them in the correct folder in your dart project. Folders for the target environments (e.g. macos, android) must be already in your project.
You can run the script from the terminal at the folder where your dart project is with the Unix command

```sh
bash <(wget -qO- https://raw.githubusercontent.com/stregato/bao/main/bindings/dart/install.sh)
```


## Usage

```dart
import 'dart:typed_data';
import 'package:bao/bao.dart';
import 'package:bao/src/bindings.dart' show bindings;

Future<void> main() async {
  // Spin up the worker isolates and load the native library.
  await bindings.start();

  final db = await DB.defaultDB();
  final privateId = newPrivateID();
  final publicId = publicID(privateId);
  final url = 'file:///tmp/$publicId/sample';
  final storeConfig = StoreConfig.fromLocalUrl(url);

  // Create a new bao
  final b = await Bao.create(db, privateId, storeConfig);

  // Grant yourself read/write access
  await b.syncAccess([AccessChange(users, accessReadWrite, publicId)]);

  // Write a file (attrs is optional metadata as bytes)
  await b.write('hello.txt', users, Uint8List(0), '/path/to/local/file.txt', 0);

  // List files
  final files = await b.readDir('', limit: 10);
  for (final f in files) {
    print('${f.name} (${f.size} bytes)');
  }

  await b.close();
  await db.close();
}
```

## Additional information

More information available on github [page](http://github.com/stregato/baolib)
