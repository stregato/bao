package ink.francesco.bao;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.List;
import java.util.Map;

public class Mailbox {

    public static class Message {
        public String subject;
        public String body;
        public List<String> attachments;
        public Map<String, Object> fileInfo;
    }

    private final Vault bao;
    private final ObjectMapper mapper = new ObjectMapper();

    public Mailbox(Vault bao) {
        this.bao = bao;
    }

    public void send(String dir, String group, Message message) throws Exception {
        Result r = BaoLibrary.instance.bao_mailbox_send(bao.hnd, dir, group, mapper.writeValueAsString(message));
        r.check();
    }

    public List<Message> receive(String dir, long since, long fromId) throws Exception {
        Result r = BaoLibrary.instance.bao_mailbox_receive(bao.hnd, dir, since, fromId);
        return r.list(Message.class);
    }

    public void download(String dir, Message message, int attachmentIndex, String dest) throws Exception {
        Result r = BaoLibrary.instance.bao_mailbox_download(bao.hnd, dir, mapper.writeValueAsString(message), attachmentIndex, dest);
        r.check();
    }
}
