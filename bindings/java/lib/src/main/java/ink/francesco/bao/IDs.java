package ink.francesco.bao;

import java.io.IOException;


public class IDs {

    static public String newPrivateID() throws IOException {
        var m = BaoLibrary.instance.bao_newPrivateID();
        return m.obj(String.class);
    }

    static public String publicID(String privateID) throws IOException {
        var m = BaoLibrary.instance.bao_publicID(privateID);
        return m.obj(String.class);
    }

    static public String decode(String id) throws IOException {
        var m = BaoLibrary.instance.bao_decodeID(id);
        return m.obj(String.class);
    }
}
