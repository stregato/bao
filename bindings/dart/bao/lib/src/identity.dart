import 'package:bao/src/bindings.dart';

typedef PrivateID = String;
typedef PublicID = String;

PrivateID newPrivateID() {
  return bindings.call('bao_newPrivateID', []).string;
}

PublicID publicID(PrivateID privateID) {
  return bindings.call('bao_publicID', [privateID]).string;
}

Map<String, dynamic> decodeID(String id) {
  return bindings.call('bao_decodeID', [id]).map; 
}
