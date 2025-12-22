import 'package:bao/bao.dart';
import 'package:test/test.dart';

void main() {
  group('A group of tests', () {
    setUp(() async {
      await initBaoLibrary();
    });

    test('Create Bao', () async {
      var i = newPrivateID();

      var db = await DB.defaultDB();

      var url = 'file:///tmp/${publicID(i)}/sample';
      var storeConfig = StoreConfig.fromLocalUrl(url);
      var s = await Bao.create(db, i, storeConfig);

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

      var url = 'file:///tmp/${publicID(i)}/sample';
      var storeConfig = StoreConfig.fromLocalUrl(url);
      var s = await Bao.create(db, i, storeConfig);
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
