//go:build !js

package store

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/pkg/sftp"
	"github.com/stregato/bao/lib/core"
	"golang.org/x/crypto/ssh"
)

type SFTP struct {
	c    *sftp.Client
	base string
	id   string
}

type SFTPConfig struct {
	Username   string `json:"username" yaml:"username"`
	Password   string `json:"password" yaml:"password"`
	Host       string `json:"host" yaml:"host"`
	Port       int    `json:"port" yaml:"port"`
	PrivateKey string `json:"keyFile" yaml:"keyFile"`
	BasePath   string `json:"basePath" yaml:"basePath"`
	Verbose    int    `json:"verbose" yaml:"verbose"`
}

// OpenSFTP create a new Exchanger. The url is in the format sftp://user:pass@host:port/basepath?k=base64encodedprivatekey
func OpenSFTP(id string, c SFTPConfig) (Store, error) {

	addr := c.Host
	if c.Port == 0 {
		addr = fmt.Sprintf("%s:22", addr)
	} else {
		addr = fmt.Sprintf("%s:%d", addr, c.Port)
	}

	var auth []ssh.AuthMethod

	password := c.Password
	if password != "" {
		auth = append(auth, ssh.Password(password))
	}

	if c.PrivateKey != "" {
		pkey, err := base64.StdEncoding.DecodeString(c.PrivateKey)
		if err != nil {
			return nil, core.Error(core.GenericError, "private key is invalid", err)
		}

		signer, err := ssh.ParsePrivateKey(pkey)
		if err != nil {
			return nil, fmt.Errorf("invalid key: %v", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}

	if len(auth) == 0 {
		return nil, fmt.Errorf("no auth method provided for sftp connection to %s", addr)
	}

	cc := &ssh.ClientConfig{
		User:            c.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	dial, err := ssh.Dial("tcp", addr, cc)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s in NewSFTP: %v", addr, err)
	}
	client, err := sftp.NewClient(dial)
	if err != nil {
		return nil, fmt.Errorf("cannot create a sftp client for %s: %v", addr, err)
	}

	base := c.BasePath
	if base == "" {
		base = "/"
	}
	return &SFTP{client, base, id}, nil
}

func (s *SFTP) ID() string {
	return s.id
}

func (s *SFTP) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	f, err := s.c.Open(path.Join(s.base, name))
	if os.IsNotExist(err) {
		return err
	}
	if err != nil {
		return core.Error(core.FileError, "cannot open file on sftp server %v:%v", s, err)
	}

	if rang == nil {
		_, err = io.Copy(dest, f)
	} else {
		left := rang.To - rang.From
		f.Seek(rang.From, 0)
		var b [4096]byte

		for left > 0 {
			var sz int64
			if rang.To-rang.From > 4096 {
				sz = 4096
			} else {
				sz = rang.To - rang.From
			}
			n, err := f.Read(b[0:sz])
			dest.Write(b[0:n])
			left -= int64(n)
			if err != nil {
				break
			}
		}
	}
	if err != io.EOF && err != nil {
		return core.Error(core.GenericError, "cannot read from %s/%s:%v", s, name, err)
	}

	return nil
}

func (s *SFTP) Write(name string, source io.ReadSeeker, progress chan int64) error {
	name = path.Join(s.base, name)

	f, err := s.c.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
	if os.IsNotExist(err) {
		dir := path.Dir(name)
		s.c.MkdirAll(dir)
		f, err = s.c.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
	}
	if err != nil {
		return core.Error(core.FileError, "cannot create SFTP file '%s'", name, err)
	}

	_, err = io.Copy(f, source)
	if err != nil {
		return core.Error(core.FileError, "cannot write SFTP file '%s'", name, err)
	}

	return nil
}

func (s *SFTP) ReadDir(dir string, f Filter) ([]fs.FileInfo, error) {
	dir = path.Join(s.base, dir)
	ls, err := s.c.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var cnt int64
	var infos []fs.FileInfo
	for _, l := range ls {
		if matchFilter(l, f) {
			infos = append(infos, l)
			cnt++
		}
		if f.MaxResults > 0 && cnt >= f.MaxResults {
			break
		}
	}

	return infos, nil
}

func (s *SFTP) Stat(name string) (os.FileInfo, error) {
	return s.c.Stat(path.Join(s.base, name))
}

func (s *SFTP) Rename(old, new string) error {
	return s.c.Rename(path.Join(s.base, old), path.Join(s.base, new))
}

func (s *SFTP) Delete(name string) error {
	n := path.Join(s.base, name)
	stat, err := s.c.Stat(n)
	if err != nil {
		return core.Error(core.DbError, "cannot stat %s in Delete", n, err)
	}

	if stat.IsDir() {
		is, _ := s.c.ReadDir(n)
		for _, i := range is {
			err = s.Delete(path.Join(name, i.Name()))
			if err != nil {
				return err
			}
		}
	}
	err = s.c.Remove(n)
	if err != nil {
		return core.Error(core.DbError, "cannot delete %s in Delete", n, err)
	}
	return nil
}

func (s *SFTP) Close() error {
	return s.c.Close()
}

func (s *SFTP) LastChange(dir string) (time.Time, error) {
	return time.Time{}, nil
}

func (s *SFTP) String() string {
	return s.id
}

// Describe implements Store.
func (*SFTP) Describe() Description {
	return Description{
		ReadCost:  0.000000001,
		WriteCost: 0.000000001,
	}
}
