import 'bindings.dart';

class PrivateID  {
  final String _value;
  
  PrivateID([String value = ""]) : _value = value.isEmpty ? bindings.call('bao_security_newPrivateID', []).string : value;

  @override
  String toString() => _value;
  
  bool get isEmpty => _value.isEmpty;
  bool get isNotEmpty => _value.isNotEmpty;
  
  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is PrivateID && other._value == _value;
  }
  
  @override
  int get hashCode => _value.hashCode;
  
  /// Decode the private ID to extract cryptKey and signKey
  Map<String, dynamic> decode() {
    return bindings.call('bao_security_decodePrivateID', [_value]).map;
  }

  PublicID publicID() {
    return PublicID(bindings.call('bao_security_publicID', [toString()]).string);
  }

  dynamic toJson() => _value;
}

class PublicID {
  final String _value;
  
  PublicID([String value=""]) : _value = value;
  
  @override
  String toString() => _value;
  
  bool get isEmpty => _value.isEmpty;
  bool get isNotEmpty => _value.isNotEmpty;
  
  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is PublicID && other._value == _value;
  }
  
  @override
  int get hashCode => _value.hashCode;
  
  /// Decode the public ID to extract cryptKey and signKey
  Map<String, dynamic> decode() {
    return bindings.call('bao_security_decodePublicID', [_value]).map;
  }

  dynamic toJson() => _value;
}
