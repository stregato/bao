package ink.francesco.bao;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;

public class Vault {

    public static String USERS = "users";
    public static String HOME = "home";
    public static String ALL = "all";

    public record AccessChange(int access, String userId) {}

    long hnd;
    String id;
    String userSecret;
    String userId;
    String url;
    Map<String, Object> storeConfig = new HashMap<>();
    String author;
    Map<String, Object> config = new HashMap<>();

    private static final ObjectMapper mapper = new ObjectMapper();

    static public Vault create(String realm, String identity, Store store, DB db, Map<String, Object> settings) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_create(realm, identity, store.hnd, db.hnd, mapper.writeValueAsString(settings));
        r.check();
        return fromResult(r);
    }

    static public Vault open(String realm, String identity, String author, Store store, DB db) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_open(realm, identity, author, store.hnd, db.hnd);
        r.check();
        return fromResult(r);
    }

    private static Vault fromResult(Result r) throws Exception {
        var s = new Vault();
        s.hnd = r.hnd;
        var m = r.map();
        s.id = (String) m.getOrDefault("id", "");
        s.userSecret = (String) m.getOrDefault("userSecret", "");
        s.userId = (String) m.getOrDefault("userId", "");
        s.url = (String) m.getOrDefault("url", "");
        Object sc = m.get("storeConfig");
        if (sc instanceof Map<?, ?> scm) {
            scm.forEach((k, v) -> s.storeConfig.put(String.valueOf(k), v));
        }
        s.author = (String) m.getOrDefault("author", "");
        Object c = m.get("config");
        if (c instanceof Map<?, ?> cm) {
            cm.forEach((k, v) -> s.config.put(String.valueOf(k), v));
        }
        return s;
    }

    public void close()  {
        BaoLibrary.instance.bao_vault_close(hnd);
    }

    public long allocatedSize() {
        Result r = BaoLibrary.instance.bao_vault_allocatedSize(hnd);
        r.check();
        try {
            return r.obj(Long.class);
        } catch (Exception e) {
            // Fallback for older native versions that return a plain string
            return Long.parseLong(r.string());
        }
    }

    public void syncAccess(List<AccessChange> changes, long options) throws JsonProcessingException {
        Result r = BaoLibrary.instance.bao_vault_syncAccess(hnd, options, mapper.writeValueAsString(changes));
        r.check();
    }

    public int getAccess(String userId) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_getAccess(hnd, userId);
        return r.integer();
    }

    public Map<String, Integer> getGroups(String user) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_getGroups(hnd, user);
        return r.map(Integer.class);
    }

    public List<String> listGroups() throws Exception {
        Result r = BaoLibrary.instance.bao_listGroups(hnd);
        return new ArrayList<>(r.list(String.class));
    }

    public List<FileInfo> waitFiles(long timeoutMs, List<Long> ids) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_waitFiles(hnd, timeoutMs, mapper.writeValueAsString(ids));
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> raw = (List<Map<String, Object>>) (List<?>) r.list(Map.class);
        return raw.stream().map(FileInfo::fromMap).collect(Collectors.toList());
    }

    public List<FileInfo> sync(List<String> groups) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_sync(hnd, mapper.writeValueAsString(groups));
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> raw = (List<Map<String, Object>>) (List<?>) r.list(Map.class);
        return raw.stream().map(FileInfo::fromMap).toList();
    }

    public void setAttribute(String name, String value, long options) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_setAttribute(hnd, options, name, value);
        r.check();
    }

    public String getAttribute(String name, String author) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_getAttribute(hnd, name, author);
        r.check();
        return r.string();
    }

    public Map<String, String> getAttributes(String author) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_getAttributes(hnd, author);
        return r.map(String.class);
    }

    public List<FileInfo> readDir(String dir, long sinceSeconds, long fromId, int limit) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_readDir(hnd, dir, sinceSeconds, fromId, limit);
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> raw = (List<Map<String, Object>>) (List<?>) r.list(Map.class);
        return raw.stream().map(FileInfo::fromMap).toList();
    }

    public FileInfo stat(String name) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_stat(hnd, name);
        return FileInfo.fromMap(r.map());
    }

    public String getAuthor(String name) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_getAuthor(hnd, name);
        return r.string();
    }

    public FileInfo read(String name, String dest, long options) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_read(hnd, name, dest, options);
        return FileInfo.fromMap(r.map());
    }

    public FileInfo write(String dest, byte[] attrs, String src, long options) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_write(hnd, dest, src, new Data(attrs), options);
        return FileInfo.fromMap(r.map());
    }

    public void delete(String name, long options) throws Exception {
        BaoLibrary.instance.bao_vault_delete(hnd, name, options).check();
    }

    public List<FileInfo> versions(String name) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_versions(hnd, name);
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> raw = (List<Map<String, Object>>) (List<?>) r.list(Map.class);
        return raw.stream().map(FileInfo::fromMap).toList();
    }
}
