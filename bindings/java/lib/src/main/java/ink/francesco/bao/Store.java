package ink.francesco.bao;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Map;

public class Store {
    long hnd;
    Map<String, Object> config;

    private static final ObjectMapper mapper = new ObjectMapper();

    public Store(long hnd, Map<String, Object> config) {
        this.hnd = hnd;
        this.config = config;
    }

    public static Store open(Map<String, Object> config) throws Exception {
        Result r = BaoLibrary.instance.bao_store_open(mapper.writeValueAsString(config));
        r.check();
        return new Store(r.hnd, config);
    }

    public void close() {
        BaoLibrary.instance.bao_store_close(hnd);
    }

    public Object readDir(String dir, Map<String, Object> filter) throws Exception {
        Result r = BaoLibrary.instance.bao_store_readDir(hnd, dir, mapper.writeValueAsString(filter));
        return r.list(Object.class);
    }

    public Map<String, Object> stat(String path) throws Exception {
        Result r = BaoLibrary.instance.bao_store_stat(hnd, path);
        return r.map();
    }

    public void delete(String path) throws Exception {
        BaoLibrary.instance.bao_store_delete(hnd, path).check();
    }
}
