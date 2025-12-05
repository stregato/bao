bao is a Dart binding for the Bao library. It enables encrypted data storage and exchange. Similar to traditional systems, Bao offers controlled access with a security model inspired by Unix permissions.
Unlike traditional storage, the access is based on cryptographic features, enabling distributed control.

## Features
Bao offers convenient  

It supports Android, iOS, macOS, Linux and Windows on the common architectures for each OS.

## Getting started
Run _flutter pub get bao_ or _dart pub get bao_ from your terminal.
Alternatevely add _bao_ dependency in your pubspec.yaml file.

The package only contains the dart source code. You still need to download the binary libraries (depending on the target architecture) from http://github.com/stregato/bao and copy to the subfolders for your architecture, i.e. _linux_, _ios_, _macos_, _android_, _windows_.

The script http://github.com/stregato/bao/dart/install.sh automatically download the libraries and place them in the correct folder in your dart project. Folders for the target environments (e.g. macos, android) must be already in your project.
You can run the script from the terminal at the folder where your dart project is with the Unix command

```sh
bash <(wget -qO- https://raw.githubusercontent.com/stregato/repo/bao/dart/install.sh)
```


## Usage


```dart
    loabaoLibrary();
    
    var i = Identity('Admin');
    var db = DB.defaultDB();

    var url = 'file:///tmp/${i.id}/sample';
    var s = Safe.create(db, i, url);

    var groups = s.getGroups();
    expect(groups, isNotNull);

    var alice = Identity('Alice');
    groups = s.updateGroup('usr', Safe.grant, [alice.id]);
    expect(groups['usr']?.contains(alice.id), true);

    groups = s.getGroups();
    expect(groups['usr']?.contains(alice.id), true);

    var keys = s.getKeys('usr');
    expect(keys, isNotNull);

    s.close();
    db.close();
```

## Additional information

More information available on github [page](http://github.com/stregato/pbao)
