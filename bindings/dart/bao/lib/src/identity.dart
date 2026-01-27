import 'package:bao/src/bindings.dart';

typedef PrivateID = String;
typedef PublicID = String;

PrivateID newPrivateID() {
  return bindings.call('bao_security_newPrivateID', []).string;
}

PublicID publicID(PrivateID privateID) {
  return bindings.call('bao_security_publicID', [privateID]).string;
}

Map<String, dynamic> decodeID(String id) {
  return bindings.call('bao_security_decodeID', [id]).map; 
}
