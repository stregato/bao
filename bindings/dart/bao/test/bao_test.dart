import 'package:bao/bao.dart';
import 'package:test/test.dart';

void main() {
  group('A group of tests', () {
    setUp(() async {
      await initBaoLibrary();
    });

    test('Create Vault', () async {
      var i = newPrivateID();

      var db = await DB.defaultDB();

      var storeConfig = StoreConfig(
        id: 'test',
        type: 'local',
        local: LocalConfig(base: '/tmp/${publicID(i)}/sample'),
      );
      var store = await Store.open(storeConfig);
      var s = await Vault.create(i, db, store);

      var alice = newPrivateID();
      var aliceID = publicID(alice);
      await s.syncAccess([AccessChange(users, accessReadWrite, aliceID)]);
      var accesses = await s.getAccess(users);
      expect(accesses[aliceID], accessReadWrite);

      s.close();
      db.close();
    });

    test('Write File', () async {
      var i = newPrivateID();
      var db = await DB.defaultDB();
      expect(db, isNotNull);

      var storeConfig = StoreConfig(
        id: 'test',
        type: 'local',
        local: LocalConfig(base: '/tmp/${publicID(i)}/sample'),
      );
      var store = await Store.open(storeConfig);
      var s = await Vault.create(i, db, store);
      expect(s, isNotNull);

      var file = await s.write('file.txt', public);
      expect(file, isNotNull);
      expect(file.name, 'file.txt');
      expect(file.size, 0);

      await s.waitFiles([file.id]);

      s.close();
      db.close();
    });
  });
}
