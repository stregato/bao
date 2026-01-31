import 'package:bao/bao.dart';

void main() async {
  await initBaoLibrary();

  var (alice, aliceSecret) = newKeyPair();
  var (bob, bobSecret) = newKeyPair();
  var db = await DB.defaultDB();

  final storeConfig = StoreConfig(
    id: 'test',
    type: 'local',
    local: LocalConfig(base: '/tmp/${alice.toString()}/sample'),
  );
  var store = await Store.open(storeConfig);
  var v = await Vault.create(users, aliceSecret, store, db);

  v.syncAccess([AccessChange(bob, accessReadWrite)]);
  print(v.getAccess(bob));

  v.close();
}
