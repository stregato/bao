import 'package:bao/bao.dart';

void main() async {
  await initBaoLibrary();

  var (alice, aliceSecret) = newKeyPair();
  var (bob, bobSecret) = newKeyPair();
  var db = await DB.defaultDB();

  final storeConfig = StoreConfig(
    id: 'test',
    type: 'local',
    local: LocalConfig(base: '/tmp/${publicID(alice)}/sample'),
  );
  var store = await Store.open(storeConfig);
  var v = await Vault.create(users, alice, db, store);

  var bobID = publicID(bob);
  v.syncAccess([AccessChange(bobID, accessReadWrite)]);
  print(v.getAccess(users));

  v.close();
}
