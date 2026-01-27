package ink.francesco.bao;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;

public class Bao {

    public static String GROUP_USERS = "users";
    public static String GROUP_ADMINS = "admins";

    public record AccessChange(String group, int access, String userId) {}

    long hnd;
    String id;
    String userId;
    String userPublicId;
    String url;
    Map<String, Object> storeConfig = new HashMap<>();
    String author;
    Map<String, Object> config = new HashMap<>();

    private static final ObjectMapper mapper = new ObjectMapper();

    static public Bao create(String identity, DB db, Store store, Map<String, Object> settings) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_create(identity, db.hnd, store.hnd, mapper.writeValueAsString(settings));
        r.check();
        return fromResult(r);
    }

    static public Bao open(String identity, DB db, Store store, Map<String, Object> settings, String author) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_open(identity, db.hnd, store.hnd, mapper.writeValueAsString(settings), author);
        r.check();
        return fromResult(r);
    }

    private static Bao fromResult(Result r) throws Exception {
        var s = new Bao();
        s.hnd = r.hnd;
        var m = r.map();
        s.id = (String) m.getOrDefault("id", "");
        s.userId = (String) m.getOrDefault("userId", "");
        s.userPublicId = (String) m.getOrDefault("userPublicId", "");
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

    public Map<String, Integer> getAccess(String groupName) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_getAccess(hnd, groupName);
        return r.map(Integer.class);
    }

    public Map<String, Integer> getGroups(String user) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_getGroups(hnd, user);
        return r.map(Integer.class);
    }

    public List<String> listGroups() throws Exception {
        Result r = BaoLibrary.instance.bao_listGroups(hnd);
        return new ArrayList<>(r.list(String.class));
    }

    public void waitFiles(List<Long> ids) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_waitFiles(hnd, mapper.writeValueAsString(ids));
        r.check();
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

    public FileInfo read(String name, String dest, long options) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_read(hnd, name, dest, options);
        return FileInfo.fromMap(r.map());
    }

    public FileInfo write(String dest, String group, byte[] attrs, String src, long options) throws Exception {
        Result r = BaoLibrary.instance.bao_vault_write(hnd, dest, src, group, new Data(attrs), options);
        return FileInfo.fromMap(r.map());
    }

    public void delete(String name, long options) throws Exception {
        BaoLibrary.instance.bao_vault_delete(hnd, name, options).check();
    }

    public SqlLayer sqlLayer(String group, DB db) throws Exception {
        Result r = BaoLibrary.instance.bao_replica_open(hnd, group, (int) db.hnd);
        r.check();
        return new SqlLayer(r.hnd);
    }

    public Mailbox mailbox() {
        return new Mailbox(this);
    }
}
