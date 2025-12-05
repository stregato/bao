import 'fileinfo.dart';

class Message {
  late String subject;
  late String body;
  late List<String> attachments;
  int? id; // Not present in Go mailbox.Message; may be derived from fileInfo
  FileInfo? fileInfo;

  Message(this.subject, this.body, {this.attachments = const [], this.id = 0});

  Message.fromJson(Map<String, dynamic> map) {
    subject = map['subject'];
    body = map['body'];
    attachments = List<String>.from(map['attachments']);
    // Optional id; Go side does not set it. If present use it, else try from fileInfo
    id = map['id'];
    final fi = map['fileInfo'];
    if (fi is Map<String, dynamic>) {
      fileInfo = FileInfo.fromMap(fi);
      id ??= fileInfo?.id; // fallback to file id
    }
  }

  toJson() {
    return {
      'subject': subject,
      'body': body,
      'attachments': attachments,
      // Intentionally omit fileInfo; Go Send populates it server-side
      if (id != null) 'id': id,
    };
  }
}