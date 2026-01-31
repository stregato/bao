package ink.francesco.bao;

import java.time.Instant;
import java.util.Base64;
import java.util.Map;

public class FileInfo {
    public long id;
    public String name;
    public long size;
    public long allocatedSize;
    public Instant modTime;
    public boolean isDir;
    public long flags;
    public byte[] attrs;
    public long keyId;
    public String storageDir;
    public String storageName;
    public String authorId;

    static FileInfo fromMap(Map<String, Object> m) {
        var fi = new FileInfo();
        fi.id = ((Number) m.getOrDefault("id", 0)).longValue();
        fi.name = (String) m.getOrDefault("name", "");
        fi.size = ((Number) m.getOrDefault("size", 0)).longValue();
        fi.allocatedSize = ((Number) m.getOrDefault("allocatedSize", 0)).longValue();
        var mod = (String) m.getOrDefault("modTime", "1970-01-01T00:00:00Z");
        fi.modTime = Instant.parse(mod);
        fi.isDir = Boolean.TRUE.equals(m.get("isDir"));
        fi.flags = ((Number) m.getOrDefault("flags", 0)).longValue();

        Object a = m.get("attrs");
        if (a instanceof String s) {
            fi.attrs = Base64.getDecoder().decode(s);
        } else if (a instanceof byte[] b) {
            fi.attrs = b;
        } else {
            fi.attrs = new byte[0];
        }
        fi.keyId = ((Number) m.getOrDefault("keyId", 0)).longValue();
        fi.storageDir = (String) m.getOrDefault("storageDir", "");
        fi.storageName = (String) m.getOrDefault("storageName", "");
        fi.authorId = (String) m.getOrDefault("authorId", "");
        return fi;
    }
}
