import 'dart:convert';
import 'dart:typed_data';

import 'package:bao/bao.dart';
import 'package:bao/src/bindings.dart';

// ignore: unused_import
import 'loader.dart';

/// Encrypt plaintext with an EC public ID (string form).
/// Returns raw ciphertext bytes.
/// Encrypts bytes with an EC public ID; returns raw ciphertext bytes.
Uint8List ecEncrypt(String publicId, Uint8List plainData) {
  return bindings.call('bao_security_ecEncrypt', [publicId, plainData]).data;
}

/// Decrypts ciphertext bytes with an EC private ID.
/// Returns raw plaintext bytes.
Uint8List ecDecrypt(String privateId, Uint8List cipherData) {
  return bindings.call('bao_security_ecDecrypt', [privateId, cipherData]).data;
}

/// Convenience: returns base64 (URL-safe, no padding) of ecEncrypt output.
String ecEncryptToBase64(String publicId, String plainText) {
	final bytes = ecEncrypt(publicId, Uint8List.fromList(utf8.encode(plainText)));
	return base64Url.encode(bytes);
}

/// Convenience: decrypt from base64 (URL-safe) text form of ciphertext.
Uint8List ecDecryptFromBase64(String privateId, String cipherBase64) {
	final data = base64Url.decode(cipherBase64);
	return ecDecrypt(privateId, data);
}

/// Encrypt plaintext bytes using AES with string key and byte nonce.
/// Returns raw ciphertext bytes.
Uint8List aesEncrypt(String key, Uint8List nonce, Uint8List plainData) {
  return bindings.call('bao_security_aesEncrypt', [key, nonce, plainData]).data;
}

/// Decrypt ciphertext bytes using AES with string key and byte nonce.
/// Returns raw plaintext bytes.
Uint8List aesDecrypt(String key, Uint8List nonce, Uint8List cipherData) {
  return bindings.call('bao_security_aesDecrypt', [key, nonce, cipherData]).data;
}

/// Helpers to work with base64 text for AES as well.
String aesEncryptToBase64(String key, Uint8List nonce, String plainText) {
	return base64Url.encode(aesEncrypt(key, nonce, Uint8List.fromList(utf8.encode(plainText))));
}

Uint8List aesDecryptFromBase64(
		String key, Uint8List nonce, String cipherBase64) {
	return aesDecrypt(key, nonce, Uint8List.fromList(base64Url.decode(cipherBase64)));
}

/// Generates a new key pair and returns a map with publicID and privateID strings.
(PublicID, PrivateID) newKeyPair() {
  var m = Map<String, String>.from(
			bindings.call('bao_security_newKeyPair', []).map);
  return (m['publicID']!, m['privateID']!);
}
