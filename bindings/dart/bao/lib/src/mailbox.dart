import 'package:bao/src/bindings.dart';
import 'package:bao/src/message.dart';

class Mailbox {
  int vaultHandle;

  Mailbox(this.vaultHandle);

  Future<void> send(String dir, String group, Message message) async {
    var res = await bindings.acall(
        'bao_mailbox_send', [vaultHandle, dir, group, message]);
    res.throwIfError();
  }

  Future<List<Message>> receive(String dir,
      {DateTime? since, int fromId = 0}) async {
    var res = await bindings.acall('bao_mailbox_receive',
        [vaultHandle, dir, since?.millisecondsSinceEpoch ?? 0, fromId]);
    return res.list.map((x) => Message.fromJson(x)).toList();
  }

  Future<void> download(
      String dir, Message message, int attachmentIndex, String dest) async {
    var res = await bindings.acall(
        'bao_mailbox_download', [vaultHandle, dir, message, attachmentIndex, dest]);
    res.throwIfError();
  }
}
