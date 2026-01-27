package ink.francesco.bao;

import com.fasterxml.jackson.annotation.JsonAlias;
import com.fasterxml.jackson.annotation.JsonProperty;

public class StoreConfig {
    public String id = "";
    public String type = "";
    public S3Config s3 = new S3Config();
    public SFTPConfig sftp = new SFTPConfig();
    public AzureConfig azure = new AzureConfig();
    public LocalConfig local = new LocalConfig();
    public WebDAVConfig webdav = new WebDAVConfig();
    public RelayConfig relay = new RelayConfig();

    public static StoreConfig s3(String id, String endpoint, String region, String bucket, String prefix,
                                 String accessKeyId, String secretAccessKey) {
        StoreConfig c = new StoreConfig();
        c.id = id;
        c.type = "s3";
        c.s3.endpoint = endpoint;
        c.s3.region = region;
        c.s3.bucket = bucket;
        c.s3.prefix = prefix;
        c.s3.auth.accessKeyId = accessKeyId;
        c.s3.auth.secretAccessKey = secretAccessKey;
        return c;
    }

    public static StoreConfig sftp(String id, String username, String password, String host, int port,
                                   String privateKey, String basePath) {
        StoreConfig c = new StoreConfig();
        c.id = id;
        c.type = "sftp";
        c.sftp.username = username;
        c.sftp.password = password;
        c.sftp.host = host;
        c.sftp.port = port;
        c.sftp.privateKey = privateKey;
        c.sftp.basePath = basePath;
        return c;
    }

    public static StoreConfig azure(String id, String accountName, String accountKey, String share,
                                    String basePath) {
        StoreConfig c = new StoreConfig();
        c.id = id;
        c.type = "azure";
        c.azure.accountName = accountName;
        c.azure.accountKey = accountKey;
        c.azure.share = share;
        c.azure.basePath = basePath;
        return c;
    }

    public static StoreConfig local(String id, String base) {
        StoreConfig c = new StoreConfig();
        c.id = id;
        c.type = "local";
        c.local.base = base;
        return c;
    }

    public static StoreConfig webdav(String id, String username, String password, String host, int port,
                                     String basePath, boolean https) {
        StoreConfig c = new StoreConfig();
        c.id = id;
        c.type = "webdav";
        c.webdav.username = username;
        c.webdav.password = password;
        c.webdav.host = host;
        c.webdav.port = port;
        c.webdav.basePath = basePath;
        c.webdav.https = https;
        return c;
    }

    public static StoreConfig relay(String id, String url, String privateId) {
        StoreConfig c = new StoreConfig();
        c.id = id;
        c.type = "relay";
        c.relay.url = url;
        c.relay.privateId = privateId;
        return c;
    }

    public static class S3ConfigAuth {
        public String accessKeyId = "";
        public String secretAccessKey = "";
    }

    public static class S3Config {
        public String endpoint = "";
        public String region = "";
        public String bucket = "";
        public String prefix = "";
        public S3ConfigAuth auth = new S3ConfigAuth();
        public int verbose = 0;
        public String proxy = "";
    }

    public static class SFTPConfig {
        public String username = "";
        public String password = "";
        public String host = "";
        public int port = 0;
        @JsonProperty("keyFile")
        @JsonAlias({"privateKey"})
        public String privateKey = "";
        public String basePath = "";
        public int verbose = 0;
    }

    public static class AzureConfig {
        public String accountName = "";
        public String accountKey = "";
        public String share = "";
        public String basePath = "";
        public int verbose = 0;
    }

    public static class LocalConfig {
        public String base = "";
    }

    public static class WebDAVConfig {
        public String username = "";
        public String password = "";
        public String host = "";
        public int port = 0;
        public String basePath = "";
        public int verbose = 0;
        public boolean https = false;
    }

    public static class RelayConfig {
        public String url = "";
        public String privateId = "";
    }
}
