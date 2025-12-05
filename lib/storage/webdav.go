package storage

import (
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/studio-b12/gowebdav"
)

type WebDAV struct {
	c   *gowebdav.Client
	p   string
	url string
}

// OpenWebDAV create a new Exchanger. The url is in the format dav://user:pass@host:port/basepath
func OpenWebDAV(connectionUrl string) (Store, error) {
	u, err := url.Parse(connectionUrl)
	if err != nil {
		return nil, core.Errorw("invalid url '%s'", connectionUrl, err)
	}

	var conn string
	switch u.Scheme {
	case "dav":
		conn = fmt.Sprintf("http://%s:%s/%s", u.Host, u.Port(), u.Path)
	case "davs":
		conn = fmt.Sprintf("https://%s:%s/%s", u.Host, u.Port(), u.Path)
	default:
		core.IsErr(os.ErrInvalid, "invalid scheme %s: %v", u.Scheme)
		return nil, os.ErrInvalid
	}

	password, _ := u.User.Password()
	c := gowebdav.NewClient(conn, u.User.Username(), password)
	err = c.Connect()
	if err != nil {
		return nil, core.Errorw("cannot connect to WebDAV '%s'", connectionUrl, err)
	}

	w := &WebDAV{
		c:   c,
		p:   u.Path,
		url: connectionUrl,
	}

	return w, nil
}

func (w *WebDAV) ID() string {
	return w.url
}

func (w *WebDAV) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	p := path.Join(w.p, name)

	r, err := w.c.ReadStream(p)
	if err != nil {
		return core.Errorw("cannot read WebDAV file %s", p, err)
	}

	var written int64
	if rang == nil {
		for err == nil {
			written, err = io.CopyN(dest, r, 1024*1024)
			if progress != nil {
				progress <- written
			}
		}
	} else {
		written, err = io.CopyN(io.Discard, r, rang.From)
		if err != nil {
			return core.Errorw("cannot discard %n bytes in range GET on %s", rang.From, p, err)
		}
		if written != rang.From {
			core.IsErr(io.ErrShortWrite, "Cannot read %d bytes in GET %s: %v")
			return io.ErrShortWrite
		}

		written, err = io.CopyN(dest, r, rang.To-rang.From)
		if progress != nil {
			progress <- written
		}
	}
	if err != nil && err != io.EOF {
		return core.Errorw("cannot read from GET response on %s", p, err)
	}
	r.Close()
	return nil
}

func (w *WebDAV) Write(name string, source io.ReadSeeker, progress chan int64) error {
	p := path.Join(w.p, name)

	err := w.c.WriteStream(p, source, 0)
	if err != nil {
		return core.Errorw("cannot write WebDAV file %s", p, err)
	}

	return nil
}

func (w *WebDAV) ReadDir(dir string, f Filter) ([]fs.FileInfo, error) {
	p := path.Join(w.p, dir)

	ls, err := w.c.ReadDir(p)
	if _, ok := err.(*fs.PathError); ok {
		return nil, os.ErrNotExist
	}
	if core.IsErr(err, "cannot read WebDAV folder %s: %v", p) {
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

	return infos, err
}

func (w *WebDAV) Stat(name string) (fs.FileInfo, error) {
	p := path.Join(w.p, name)

	f, err := w.c.Stat(p)
	if _, ok := err.(*fs.PathError); ok {
		return nil, os.ErrNotExist
	}

	if err != nil {
		return nil, core.Errorw("cannot read WebDAV folder %s", p, err)
	}
	return f, err
}

func (w *WebDAV) Rename(old, new string) error {
	o := path.Join(w.p, old)
	n := path.Join(w.p, new)
	return w.c.Rename(o, n, true)
}

func (w *WebDAV) Delete(name string) error {
	p := path.Join(w.p, name)
	return w.c.RemoveAll(p)
}

func (w *WebDAV) Close() error {
	return nil
}

func (w *WebDAV) LastChange(dir string) (time.Time, error) {
	st, err := w.c.Stat(path.Join(w.p, dir))
	if os.IsNotExist(err) {
		return time.Time{}, nil
	}
	return st.ModTime(), err
}

func (w *WebDAV) String() string {
	return w.url
}

func (w *WebDAV) Describe() Description {
	return Description{
		ReadCost:  0,
		WriteCost: 0,
	}
}
