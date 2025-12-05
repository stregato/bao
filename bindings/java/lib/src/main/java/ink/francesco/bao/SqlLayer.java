package ink.francesco.bao;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Iterator;
import java.util.Map;

public class SqlLayer {

    private final long hnd;
    private final ObjectMapper mapper = new ObjectMapper();

    protected SqlLayer(long hnd) {
        this.hnd = hnd;
    }

    public Map<String, Object> exec(String sql, Map<String, Object> args) throws Exception {
        Result res = BaoLibrary.instance.baoql_exec(hnd, sql, mapper.writeValueAsString(args));
        return res.map();
    }

    public Rows query(String sql, Map<String, Object> args) throws Exception {
        Result res = BaoLibrary.instance.baoql_query(hnd, sql, mapper.writeValueAsString(args));
        res.check();
        return new Rows(res.hnd);
    }

    public void syncTables() {
        BaoLibrary.instance.baoql_sync_tables(hnd).check();
    }

    public void cancel() {
        BaoLibrary.instance.baoql_cancel(hnd).check();
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
                Result r = BaoLibrary.instance.baoql_next(rowsHandle);
                var err = r.getError();
                if (err != null || r.hnd == 0) {
                    return false;
                }
                next = r.map();
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
            Result r = BaoLibrary.instance.baoql_current(rowsHandle);
            r.check();
            return r.map();
        }

        public void close() throws Exception {
            BaoLibrary.instance.baoql_closeRows(rowsHandle).check();
        }
    }
}
