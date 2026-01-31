package ink.francesco.bao;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Iterator;
import java.util.Map;

public class Replica {

    private final long hnd;
    private final ObjectMapper mapper = new ObjectMapper();

    public Replica(Vault vault, DB db) throws Exception {
        Result r = BaoLibrary.instance.bao_replica_open(vault.hnd, (int) db.hnd);
        r.check();
        this.hnd = r.hnd;
    }

    public Map<String, Object> exec(String sql, Map<String, Object> args) throws Exception {
        Result res = BaoLibrary.instance.bao_replica_exec(hnd, sql, mapper.writeValueAsString(args));
        return res.map();
    }

    public Rows query(String sql, Map<String, Object> args) throws Exception {
        Result res = BaoLibrary.instance.bao_replica_query(hnd, sql, mapper.writeValueAsString(args));
        res.check();
        return new Rows(res.hnd);
    }

    public void syncTables() {
        BaoLibrary.instance.bao_replica_sync(hnd).check();
    }

    public void cancel() {
        BaoLibrary.instance.bao_replica_cancel(hnd).check();
    }

    public void close() {
        BaoLibrary.instance.bao_replica_cancel(hnd).check();
    }

    public class Rows implements Iterator<Map<String, Object>>, Iterable<Map<String, Object>> {
        private long rowsHandle;
        private Map<String, Object> next;

        public Rows(long rowsHandle) {
            this.rowsHandle = rowsHandle;
        }

        @Override
        public boolean hasNext() {
            try {
                Result r = BaoLibrary.instance.bao_replica_next(rowsHandle);
                r.check();
                boolean more = r.obj(Boolean.class);
                if (!more) {
                    return false;
                }
                next = current();
                return true;
            } catch (Exception e) {
                return false;
            }
        }

        @Override
        public Map<String, Object> next() {
            return next;
        }

        @Override
        public Iterator<Map<String, Object>> iterator() {
            return this;
        }

        public Map<String, Object> current() throws Exception {
            Result r = BaoLibrary.instance.bao_replica_current(rowsHandle);
            r.check();
            return r.map();
        }

        public void close() throws Exception {
            BaoLibrary.instance.bao_replica_closeRows(rowsHandle).check();
        }
    }
}
