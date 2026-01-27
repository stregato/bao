package ink.francesco.bao;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Map;

public class DB {
    long hnd;

    static public DB defaultDB() throws Exception {
        String osName = System.getProperty("os.name").toLowerCase();
        String userHome = System.getProperty("user.home");
        Path configFolder;

        if (osName.contains("win")) {
            String appData = System.getenv("APPDATA");
            configFolder = Paths.get(appData, "bao.db");
        } else if (osName.contains("mac")) {
            configFolder = Paths.get(userHome, "Library", "Application Support", "bao.db");
        } else {
            configFolder = Paths.get(userHome, ".config", "bao.db");
        }
        return new DB(configFolder.toString());
    }

    public DB(String path) throws Exception {
        Result r = BaoLibrary.instance.bao_db_open("sqlite3", path, "");
        r.check();
        hnd = r.hnd;
    }

    public void close() {
        BaoLibrary.instance.bao_db_close(hnd);
    }

    public Map<String, Object> query(String key, String argsJson) throws Exception {
        var r = BaoLibrary.instance.bao_db_query(hnd, key, argsJson);
        return r.map();
    }

    public Map<String, Object> exec(String key, String argsJson) throws Exception {
        var r = BaoLibrary.instance.bao_db_exec(hnd, key, argsJson);
        return r.map();
    }

    public Map<String, Object> fetch(String key, String argsJson, int maxRows) throws Exception {
        var r = BaoLibrary.instance.bao_db_fetch(hnd, key, argsJson, maxRows);
        return r.map();
    }

    public Map<String, Object> fetchOne(String key, String argsJson) throws Exception {
        var r = BaoLibrary.instance.bao_db_fetch_one(hnd, key, argsJson);
        return r.map();
    }
}
