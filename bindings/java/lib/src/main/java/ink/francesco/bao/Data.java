package ink.francesco.bao;

import java.util.Arrays;
import java.util.List;

import com.sun.jna.Memory;
import com.sun.jna.NativeLong;
import com.sun.jna.Pointer;
import com.sun.jna.Structure;

public class Data extends Structure implements Structure.ByValue {
    public Pointer ptr;
    public NativeLong len;

    @Override
    protected List<String> getFieldOrder() {
        return Arrays.asList("ptr", "len");
    }

    public byte[] getData() {
        return ptr == null ? null : ptr.getByteArray(0, (int) len.longValue());
    }

    public Data(byte[] data) {
        if (data != null && data.length > 0) {
            Memory memory = new Memory(data.length); // Allocate memory for the data
            memory.write(0, data, 0, data.length); // Write data to memory
            this.ptr = memory; // Assign memory to the pointer
            this.len = new NativeLong(data.length); // Set the length
        } else {
            this.ptr = null;
            this.len = new NativeLong(0);
        }
    }
}
  
