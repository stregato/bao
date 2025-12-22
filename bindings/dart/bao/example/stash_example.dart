import 'package:bao/bao.dart';

void main() async {
  initBaoLibrary();

  var alice = newPrivateID();
  var db = await DB.defaultDB();

  final storeConfig =
      StoreConfig.fromLocalUrl('file:///tmp/${publicID(alice)}/sample');
  var s = await Bao.create(db, alice, storeConfig);
  var bob = newPrivateID();

  var bobID = publicID(bob);
  s.syncAccess([AccessChange(users, accessReadWrite, bobID)]);
  print(s.getAccess(users));

  s.close();
}
