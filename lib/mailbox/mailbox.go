package mailbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/bao"
)

type Message struct {
	Subject     string      `json:"subject"`
	Body        string      `json:"body"`
	Attachments []string    `json:"attachments"`
	FileInfo    *bao.File `json:"fileInfo"`
}

func Send(s *bao.Bao, dir string, group bao.Group, message Message) error {
	id := core.SnowIDString()

	res := make(chan error, len(message.Attachments))
	for idx, attachment := range message.Attachments {
		go func(idx int, attachment string) {
			name := path.Join(dir, fmt.Sprintf("%s/%40x.attachment", id, idx))
			_, err := s.Write(name, attachment, group, nil, 0, nil)
			res <- err
			message.Attachments[idx] = filepath.Base(attachment)
		}(idx, attachment)
	}
	for i := 0; i < len(message.Attachments); i++ {
		if err := <-res; err != nil {
			return err
		}
	}

	attrs, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = s.Write(path.Join(dir, id), "", group, attrs, 0, nil)
	if err != nil {
		return err
	}

	return nil
}

func Receive(s *bao.Bao, dir string, since time.Time, fromLocalId int64) ([]Message, error) {
	ls, err := s.ReadDir(dir, since, fromLocalId, 0)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(ls, func(i, j int) bool {
		return ls[i].ModTime.Before(ls[j].ModTime)
	})

	var messages []Message
	for _, fi := range ls {
		if fi.IsDir || fi.Flags&bao.Deleted != 0 || fi.Attrs == nil {
			continue
		}

		var message Message
		err = json.Unmarshal(fi.Attrs, &message)
		if err != nil {
			continue
		}
		message.FileInfo = &fi

		messages = append(messages, message)
	}

	return messages, nil
}

func Download(s *bao.Bao, dir string, m Message, attachment int, dest string) error {
	name := path.Join(dir, fmt.Sprintf("%s/%40x.attachment", m.FileInfo.Name, attachment))
	_, err := s.Read(path.Join(dir, name), dest, 0, nil)
	if err != nil {
		return err
	}
	return nil
}
