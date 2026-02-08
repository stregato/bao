import 'package:bao/bao.dart';
import 'package:test/test.dart';

void main() {
  group('A group of tests', () {
    setUp(() async {
      await initBaoLibrary();
    });

    test('Create Vault', () async {
      var idSecret = PrivateID();
      var db = await DB.defaultDB();

      var storeConfig = StoreConfig(
        id: 'test',
        type: 'local',
        local: LocalConfig(base: '/tmp/${idSecret.publicID()}/sample'),
      );
      var store = await Store.open(storeConfig);
      var s = await Vault.create(users, idSecret, store, db);

      var (alice, aliceSecret) = newKeyPair();
      await s.syncAccess([AccessChange(alice, accessReadWrite)]);
      var access = await s.getAccess(alice);
      expect(access, accessReadWrite);

      s.close();
      db.close();
    });

    test('Write File', () async {
      var idSecret = PrivateID();
      var db = await DB.defaultDB();
      expect(db, isNotNull);

      var storeConfig = StoreConfig(
        id: 'test',
        type: 'local',
        local: LocalConfig(base: '/tmp/${idSecret.publicID()}/sample'),
      );
      var store = await Store.open(storeConfig);
      var s = await Vault.create(users, idSecret, store, db);
      expect(s, isNotNull);

      var file = await s.write('file.txt');
      expect(file, isNotNull);
      expect(file.name, 'file.txt');
      expect(file.size, 0);

      await s.waitFiles(0, [file.id]);

      s.close();
      db.close();
    });
  });
}
