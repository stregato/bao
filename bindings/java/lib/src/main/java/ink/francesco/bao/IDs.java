package ink.francesco.bao;

import java.io.IOException;
import java.util.Map;


public class IDs {

    static public String newPrivateID() throws IOException {
        var m = BaoLibrary.instance.bao_security_newPrivateID();
        return m.obj(String.class);
    }

    static public String publicID(String privateID) throws IOException {
        var m = BaoLibrary.instance.bao_security_publicID(privateID);
        return m.obj(String.class);
    }

    static public Map<String, Object> decode(String id) throws IOException {
        var m = BaoLibrary.instance.bao_security_decodeID(id);
        return m.map();
    }
}
