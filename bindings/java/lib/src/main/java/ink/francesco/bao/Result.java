package ink.francesco.bao;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.jna.NativeLong;
import com.sun.jna.Pointer;
import com.sun.jna.Structure;

public class Result extends Structure implements Structure.ByValue {
    public Pointer ptr;
    public NativeLong len;
    public long hnd;
    public Pointer err;
    private final ObjectMapper mapper = new ObjectMapper();

    @Override
    protected List<String> getFieldOrder() {
        return Arrays.asList("ptr", "len", "hnd", "err");
    }

    private byte[] data() {
        return ptr == null ? null : ptr.getByteArray(0, (int) len.longValue());
    }

    public void check() {
        var e = getError();
        if (e != null) {
            throw new RuntimeException(e);
        }
    }

    public String getError() {
        return err == null ? null : err.getString(0);
    }

    public Map<String, Object> map() throws IOException {
        check();
        return mapper.readValue(data(), mapper.getTypeFactory().constructMapType(Map.class, String.class, Object.class));
    }

    public <T> Map<String, T> map(Class<T> valueClass) throws IOException {
        check();
        return mapper.readValue(data(), mapper.getTypeFactory().constructMapType(Map.class, String.class, valueClass));
    }

    public <T> List<T> list(Class<T> clazz) throws IOException {
        check();
        var bytes = data();
        List<T> list = mapper.readValue(bytes, mapper.getTypeFactory().constructCollectionType(List.class, clazz));
        return list == null ? new ArrayList<>() : list;
    }

    public <T> T obj(Class<T> clazz) throws IOException {
        check();
        return mapper.readValue(data(), clazz);
    }

    public String string() {
        check();
        var bytes = data();
        return bytes == null ? "" : new String(bytes);
    }

    public byte[] bytes() {
        check();
        return data();
    }
}


// public class Result {
//     private final long hnd;
//     private final byte[] data;
//     private final String error;

//     public Result(long hnd, byte[] data, String error) {
//         this.hnd = hnd;
//         this.data = data;
//         this.error = error;
//     }

//     public long getHnd() {
//         return hnd;
//     }

//     public byte[] getData() {
//         return data;
//     }

//     public String getError() {
//         return error;
//     }

//     public void check() {
//         if (error != null) {
//             throw new RuntimeException(error);
//         }
//     }

//     public Map<String, Object> map() throws IOException {
//         check();
//         if (data == null) {
//             return null;
//         }
//         return new ObjectMapper().readValue(data, Map.class);
//     }

//     public List<Object> list()  throws IOException {
//         check();
//         if (data == null) {
//             return null;
//         }
//             return new ObjectMapper().readValue(data, List.class);
//     }

//     @Override
//     public String toString() {
//         return "Result{" +
//                 "hnd=" + hnd +
//                 ", data=" + (data != null ? new String(data) : "null") +
//                 ", error='" + error + '\'' +
//                 '}';
//     }
// }
