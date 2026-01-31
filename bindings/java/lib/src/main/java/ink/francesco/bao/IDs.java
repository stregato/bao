package ink.francesco.bao;

import java.io.IOException;
import java.util.Map;


public class IDs {

    public static PrivateID newPrivateID() throws IOException {
        var m = BaoLibrary.instance.bao_security_newPrivateID();
        return new PrivateID(m.obj(String.class));
    }

    public static KeyPair newKeyPair() throws IOException {
        var m = BaoLibrary.instance.bao_security_newKeyPair();
        var map = m.map();
        return new KeyPair(
            new PublicID((String) map.get("publicID")),
            new PrivateID((String) map.get("privateID"))
        );
    }

    public static PublicID publicID(PrivateID privateID) throws IOException {
        var m = BaoLibrary.instance.bao_security_publicID(privateID.toString());
        return new PublicID(m.obj(String.class));
    }

    public static class PublicID {
        private final String value;

        public PublicID(String value) {
            this.value = value;
        }

        public Map<String, Object> decode() throws IOException {
            var m = BaoLibrary.instance.bao_security_decodePublicID(value);
            return m.map();
        }

        @Override
        public String toString() {
            return value;
        }

        @Override
        public boolean equals(Object obj) {
            if (this == obj) return true;
            if (obj == null || getClass() != obj.getClass()) return false;
            PublicID other = (PublicID) obj;
            return value.equals(other.value);
        }

        @Override
        public int hashCode() {
            return value.hashCode();
        }
    }

    public static class PrivateID {
        private final String value;

        public PrivateID(String value) {
            this.value = value;
        }

        public Map<String, Object> decode() throws IOException {
            var m = BaoLibrary.instance.bao_security_decodePrivateID(value);
            return m.map();
        }

        @Override
        public String toString() {
            return value;
        }

        @Override
        public boolean equals(Object obj) {
            if (this == obj) return true;
            if (obj == null || getClass() != obj.getClass()) return false;
            PrivateID other = (PrivateID) obj;
            return value.equals(other.value);
        }

        @Override
        public int hashCode() {
            return value.hashCode();
        }
    }

    public record KeyPair(PublicID publicID, PrivateID privateID) {}
}
