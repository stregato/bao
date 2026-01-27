
import 'package:bao/src/bindings.dart';

class S3ConfigAuth {
  final String accessKeyId;
  final String secretAccessKey;

  const S3ConfigAuth({this.accessKeyId = '', this.secretAccessKey = ''});

  factory S3ConfigAuth.fromJson(Map<String, dynamic> json) {
    return S3ConfigAuth(
      accessKeyId: json['accessKeyId'] ?? '',
      secretAccessKey: json['secretAccessKey'] ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'accessKeyId': accessKeyId,
      'secretAccessKey': secretAccessKey,
    };
  }
}

class S3Config {
  final String endpoint;
  final String region;
  final String bucket;
  final String prefix;
  final S3ConfigAuth auth;
  final int verbose;
  final String proxy;

  const S3Config({
    this.endpoint = '',
    this.region = '',
    this.bucket = '',
    this.prefix = '',
    this.auth = const S3ConfigAuth(),
    this.verbose = 0,
    this.proxy = '',
  });

  factory S3Config.fromJson(Map<String, dynamic> json) {
    return S3Config(
      endpoint: json['endpoint'] ?? '',
      region: json['region'] ?? '',
      bucket: json['bucket'] ?? '',
      prefix: json['prefix'] ?? '',
      auth: json['auth'] != null
          ? S3ConfigAuth.fromJson(json['auth'] as Map<String, dynamic>)
          : const S3ConfigAuth(),
      verbose: json['verbose'] ?? 0,
      proxy: json['proxy'] ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'endpoint': endpoint,
      'region': region,
      'bucket': bucket,
      'prefix': prefix,
      'auth': auth.toJson(),
      'verbose': verbose,
      'proxy': proxy,
    };
  }
}

class SFTPConfig {
  final String username;
  final String password;
  final String host;
  final int port;
  final String privateKey;
  final String basePath;
  final int verbose;

  const SFTPConfig({
    this.username = '',
    this.password = '',
    this.host = '',
    this.port = 0,
    this.privateKey = '',
    this.basePath = '',
    this.verbose = 0,
  });

  factory SFTPConfig.fromJson(Map<String, dynamic> json) {
    return SFTPConfig(
      username: json['username'] ?? '',
      password: json['password'] ?? '',
      host: json['host'] ?? '',
      port: json['port'] ?? 0,
      privateKey: json['keyFile'] ?? json['privateKey'] ?? '',
      basePath: json['basePath'] ?? '',
      verbose: json['verbose'] ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'username': username,
      'password': password,
      'host': host,
      'port': port,
      'keyFile': privateKey,
      'basePath': basePath,
      'verbose': verbose,
    };
  }
}

class AzureConfig {
  final String accountName;
  final String accountKey;
  final String share;
  final String basePath;
  final int verbose;

  const AzureConfig({
    this.accountName = '',
    this.accountKey = '',
    this.share = '',
    this.basePath = '',
    this.verbose = 0,
  });

  factory AzureConfig.fromJson(Map<String, dynamic> json) {
    return AzureConfig(
      accountName: json['accountName'] ?? '',
      accountKey: json['accountKey'] ?? '',
      share: json['share'] ?? '',
      basePath: json['basePath'] ?? '',
      verbose: json['verbose'] ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'accountName': accountName,
      'accountKey': accountKey,
      'share': share,
      'basePath': basePath,
      'verbose': verbose,
    };
  }
}

class LocalConfig {
  final String base;

  const LocalConfig({this.base = ''});

  factory LocalConfig.fromJson(Map<String, dynamic> json) {
    return LocalConfig(base: json['base'] ?? '');
  }

  Map<String, dynamic> toJson() => {'base': base};
}

class WebDAVConfig {
  final String username;
  final String password;
  final String host;
  final int port;
  final String basePath;
  final int verbose;
  final bool https;

  const WebDAVConfig({
    this.username = '',
    this.password = '',
    this.host = '',
    this.port = 0,
    this.basePath = '',
    this.verbose = 0,
    this.https = true,
  });

  factory WebDAVConfig.fromJson(Map<String, dynamic> json) {
    return WebDAVConfig(
      username: json['username'] ?? '',
      password: json['password'] ?? '',
      host: json['host'] ?? '',
      port: json['port'] ?? 0,
      basePath: json['basePath'] ?? '',
      verbose: json['verbose'] ?? 0,
      https: json['https'] ?? true,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'username': username,
      'password': password,
      'host': host,
      'port': port,
      'basePath': basePath,
      'verbose': verbose,
      'https': https,
    };
  }
}

class RelayConfig {
  final String url;
  final String privateId;

  const RelayConfig({this.url = '', this.privateId = ''});

  factory RelayConfig.fromJson(Map<String, dynamic> json) {
    return RelayConfig(
      url: json['url'] ?? '',
      privateId: json['privateId'] ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'url': url,
      'privateId': privateId,
    };
  }
}

class StoreConfig {
  final String id;
  final String type;
  final S3Config s3;
  final SFTPConfig sftp;
  final AzureConfig azure;
  final LocalConfig local;
  final WebDAVConfig webdav;
  final RelayConfig relay;

  const StoreConfig({
    this.id = '',
    this.type = '',
    this.s3 = const S3Config(),
    this.sftp = const SFTPConfig(),
    this.azure = const AzureConfig(),
    this.local = const LocalConfig(),
    this.webdav = const WebDAVConfig(),
    this.relay = const RelayConfig(),
  });

  factory StoreConfig.fromJson(Map<String, dynamic> json) {
    return StoreConfig(
      id: json['id'] ?? '',
      type: json['type'] ?? '',
      s3: json['s3'] != null
          ? S3Config.fromJson(json['s3'] as Map<String, dynamic>)
          : const S3Config(),
      sftp: json['sftp'] != null
          ? SFTPConfig.fromJson(json['sftp'] as Map<String, dynamic>)
          : const SFTPConfig(),
      azure: json['azure'] != null
          ? AzureConfig.fromJson(json['azure'] as Map<String, dynamic>)
          : const AzureConfig(),
      local: json['local'] != null
          ? LocalConfig.fromJson(json['local'] as Map<String, dynamic>)
          : const LocalConfig(),
      webdav: json['webdav'] != null
          ? WebDAVConfig.fromJson(json['webdav'] as Map<String, dynamic>)
          : const WebDAVConfig(),
      relay: json['relay'] != null
          ? RelayConfig.fromJson(json['relay'] as Map<String, dynamic>)
          : const RelayConfig(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'type': type,
      's3': s3.toJson(),
      'sftp': sftp.toJson(),
      'azure': azure.toJson(),
      'local': local.toJson(),
      'webdav': webdav.toJson(),
      'relay': relay.toJson(),
    };
  }
}

class Store {
  int hnd = 0;
  StoreConfig config = const StoreConfig();

  Store(this.hnd, this.config);

  static Future<Store> open(StoreConfig config) async {
    var res = await bindings.acall('bao_store_open', [config.toJson()]);
    res.throwIfError();
    return Store(res.handle, config);
  }

  Future<void> close() async {
    var res = await bindings.acall('bao_store_close', [hnd]);
    res.throwIfError();
  }

  Future<List<dynamic>> readDir(String path,
      {Map<String, dynamic> filter = const {}}) async {
    var res = await bindings.acall('bao_store_readDir', [hnd, path, filter]);
    return res.list;
  }

  Future<Map<String, dynamic>> stat(String path) async {
    var res = await bindings.acall('bao_store_stat', [hnd, path]);
    return res.map;
  }

  Future<void> delete(String path) async {
    var res = await bindings.acall('bao_store_delete', [hnd, path]);
    res.throwIfError();
  }
}
