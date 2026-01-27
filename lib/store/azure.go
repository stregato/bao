//go:build !js

package store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-file-go/azfile"
	
	"github.com/stregato/bao/lib/core"
)

// Addr        string
// AccountName string
// AccountKey  string
// Share       string

type Azure struct {
	p       pipeline.Pipeline
	id      string
	dir     string
	baseURL *url.URL
}

type AzureConfig struct {
	AccountName string `json:"accountName" yaml:"accountName"`
	AccountKey  string `json:"accountKey" yaml:"accountKey"`
	Share       string `json:"share" yaml:"share"`
	BasePath    string `json:"basePath" yaml:"basePath"`
	Verbose     int    `json:"verbose" yaml:"verbose"`
}

// OpenAzure create a new Exchanger. The url is in the format azure://accountname.file.core.windows.net/share/basepath?a=accountname&k=accountkey
func OpenAzure(id string, c AzureConfig) (Store, error) {
	if c.Share == "" {
		return nil, core.Errorw("azure share is missing for %s", id)
	}

	endpoint := fmt.Sprintf("https://%s.file.core.windows.net", c.AccountName)
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, core.Errorw("invalid azure endpoint %s", endpoint, err)
	}

	dir := c.Share
	if bp := strings.TrimLeft(c.BasePath, "/"); bp != "" {
		dir = path.Join(c.Share, bp)
	}

	credential, err := azfile.NewSharedKeyCredential(c.AccountName, c.AccountKey)
	if err != nil {
		return nil, core.Errorw("cannot create Azure credential for %s", id, err)
	}

	p := azfile.NewPipeline(credential, azfile.PipelineOptions{})

	a := &Azure{
		p:       p,
		id:      id,
		dir:     dir,
		baseURL: baseURL,
	}
	return a, nil
}

func (a *Azure) ID() string {
	return a.id
}

func (a *Azure) MkdirAll(name string) error {
	if name == "" {
		return nil
	}
	ctx := context.Background()
	directoryUrl, err := a.getDirectoryUrl(name)
	if err != nil {
		return err
	}
	_, err = directoryUrl.GetProperties(ctx)
	if err == nil {
		return nil
	}

	d := ""
	for _, p := range strings.Split(name, "/") {
		directoryUrl, _ = a.getDirectoryUrl(d)
		directoryUrl = directoryUrl.NewDirectoryURL(p)
		_, err = directoryUrl.Create(ctx, azfile.Metadata{}, azfile.SMBProperties{})
		if err != nil {
			return err
		}
		d = path.Join(d, p)
	}
	return nil
}

func (a *Azure) getFileUrl(name string) (azfile.FileURL, error) {
	relPath := path.Join(a.dir, name)
	rel := &url.URL{Path: relPath}
	u := a.baseURL.ResolveReference(rel)
	return azfile.NewFileURL(*u, a.p), nil
}

func (a *Azure) getDirectoryUrl(name string) (azfile.DirectoryURL, error) {
	relPath := path.Join(a.dir, name)
	rel := &url.URL{Path: relPath}
	u := a.baseURL.ResolveReference(rel)
	return azfile.NewDirectoryURL(*u, a.p), nil
}

func (a *Azure) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	ctx := context.Background()
	defer ctx.Done()

	var offset int64
	var count int64 = azfile.CountToEnd

	if rang != nil {
		offset = rang.From
		count = rang.To - rang.From
	}

	fileURL, err := a.getFileUrl(name)
	if err != nil {
		return err
	}

	resp, err := fileURL.Download(ctx, offset, count, false)
	if err != nil {
		return err
	}
	r := resp.Body(azfile.RetryReaderOptions{MaxRetryRequests: 3})
	defer r.Close()

	_, err = io.Copy(dest, r)
	return err
}

func (a *Azure) Write(name string, source io.ReadSeeker, progress chan int64) error {
	ctx := context.Background()
	defer ctx.Done()

	_ = a.MkdirAll(path.Dir(name))

	fileURL, err := a.getFileUrl(name)
	if err != nil {
		return err
	}
	_, err = fileURL.Create(ctx, azfile.FileMaxSizeInBytes, azfile.FileHTTPHeaders{}, azfile.Metadata{})
	if err != nil {
		return err
	}

	var offset int64
	var n int
	buf := make([]byte, 16000)
	for err != io.EOF {
		n, err = source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n > 0 {
			body := bytes.NewReader(buf[0:n])
			_, err = fileURL.UploadRange(ctx, offset, body, nil)
			if err != nil {
				_, _ = fileURL.Resize(ctx, 0)
				return err
			}
			offset += int64(n)
		}
	}

	_, err = fileURL.Resize(ctx, offset)
	return err
}

func (az *Azure) ReadDir(dir string, f Filter) ([]fs.FileInfo, error) {
	ctx := context.Background()
	defer ctx.Done()

	directoryURL, err := az.getDirectoryUrl(dir)
	if err != nil {
		return nil, err
	}

	ls, err := directoryURL.ListFilesAndDirectoriesSegment(ctx, azfile.Marker{},
		azfile.ListFilesAndDirectoriesOptions{})
	if err != nil {
		return nil, err
	}
	var infos []fs.FileInfo

	if !f.OnlyFiles {
		for _, l := range ls.DirectoryItems {
			n := l.Name
			info := simpleFileInfo{
				name:  n,
				isDir: true,
			}
			if matchFilter(info, f) {
				infos = append(infos, info)
			}
		}
	}
	if !f.OnlyFolders {
		for _, l := range ls.FileItems {
			n := l.Name
			fileUrl := directoryURL.NewFileURL(n)
			props, err := fileUrl.GetProperties(ctx)
			if err != nil {
				return nil, err
			}
			info := simpleFileInfo{
				name:    n,
				size:    props.ContentLength(),
				isDir:   false,
				modTime: props.LastModified(),
			}
			if matchFilter(info, f) {
				infos = append(infos, info)
			}
		}
	}

	return infos, nil
}

func (a *Azure) Stat(name string) (fs.FileInfo, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fileUrl, err := a.getFileUrl(name)
	if err != nil {
		return nil, err
	}
	properties, err := fileUrl.GetProperties(ctx)
	if err != nil {
		return nil, err
	}

	return simpleFileInfo{
		name:    path.Base(name),
		size:    properties.ContentLength(),
		isDir:   false,
		modTime: properties.LastModified(),
	}, nil
}

func (a *Azure) Rename(old, new string) error {
	oldUrl, err := a.getFileUrl(old)
	if err != nil {
		return err
	}
	newUrl, err := a.getFileUrl(new)
	if err != nil {
		return err
	}

	ctx := context.Background()

	_, err = newUrl.Create(ctx, azfile.FileMaxSizeInBytes, azfile.FileHTTPHeaders{}, azfile.Metadata{})
	if err != nil {
		return err
	}

	_, err = newUrl.StartCopy(ctx, oldUrl.URL(), azfile.Metadata{})
	if err != nil {
		return err
	}
	_, _ = oldUrl.Delete(ctx)
	return err
}

func (a *Azure) Delete(name string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fileUrl, err := a.getFileUrl(name)
	if err != nil {
		return err
	}
	_, err = fileUrl.Delete(ctx)
	return err
}

func (a *Azure) LastChange(dir string) (time.Time, error) {
	return time.Time{}, nil
}

func (a *Azure) Close() error {
	return nil
}

func (a *Azure) String() string {
	return a.id
}

// Describe implements Store.
func (*Azure) Describe() Description {
	return Description{
		ReadCost:  0.00011,
		WriteCost: 0.000005,
	}
}
