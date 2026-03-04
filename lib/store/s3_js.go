//go:build js

package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/stregato/bao/lib/core"
)

type S3 struct {
	endpoint string
	region   string
	bucket   string
	id       string
	prefix   string
	proxy    string
	creds    aws.Credentials
	signer   *v4.Signer
}

type S3ConfigAuth struct {
	AccessKeyId     string `json:"accessKeyId" yaml:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey" yaml:"secretAccessKey"`
	SessionToken    string `json:"sessionToken" yaml:"sessionToken"`
}

type S3Config struct {
	Endpoint string       `json:"endpoint" yaml:"endpoint"`
	Region   string       `json:"region" yaml:"region"`
	Bucket   string       `json:"bucket" yaml:"bucket"`
	Prefix   string       `json:"prefix" yaml:"prefix"`
	Auth     S3ConfigAuth `json:"auth" yaml:"auth"`
	Verbose  int          `json:"verbose" yaml:"verbose"`
	Proxy    string       `json:"proxy" yaml:"proxy"`
}

type s3ListV2Result struct {
	XMLName        xml.Name `xml:"ListBucketResult"`
	CommonPrefixes []struct {
		Prefix string `xml:"Prefix"`
	} `xml:"CommonPrefixes"`
	Contents []struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		Size         int64  `xml:"Size"`
	} `xml:"Contents"`
}

func OpenS3(id string, c S3Config) (Store, error) {
	core.Start("config %v", c)
	if strings.TrimSpace(c.Endpoint) == "" {
		return nil, core.Error(core.ConfigError, "endpoint is required for js s3 store")
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return nil, core.Error(core.ConfigError, "bucket is required for js s3 store")
	}
	if strings.TrimSpace(c.Auth.AccessKeyId) == "" || strings.TrimSpace(c.Auth.SecretAccessKey) == "" {
		return nil, core.Error(core.ConfigError, "accessKeyId and secretAccessKey are required for js s3 store")
	}
	if c.Region == "" {
		c.Region = "auto"
	}

	s := &S3{
		endpoint: strings.TrimRight(c.Endpoint, "/"),
		region:   c.Region,
		bucket:   c.Bucket,
		id:       id,
		prefix:   c.Prefix,
		proxy:    strings.TrimSpace(c.Proxy),
		creds: aws.Credentials{
			AccessKeyID:     c.Auth.AccessKeyId,
			SecretAccessKey: c.Auth.SecretAccessKey,
			SessionToken:    c.Auth.SessionToken,
		},
		signer: v4.NewSigner(),
	}

	core.Info("skip bucket existence/provisioning checks for js runtime (bucket %s)", c.Bucket)
	core.End("")
	return s, nil
}

func (s *S3) ID() string { return s.id }

func (s *S3) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	core.Start("name %s, rang %v", name, rang)
	key := path.Join(s.prefix, name)
	u := s.objectURL(key)
	headers := map[string]string{}
	if rang != nil {
		if rang.To > 0 {
			headers["Range"] = fmt.Sprintf("bytes=%d-%d", rang.From, rang.To)
		} else {
			headers["Range"] = fmt.Sprintf("bytes=%d-", rang.From)
		}
	}
	res, err := s.signedFetch("GET", u, headers, nil)
	if err != nil {
		return core.Error(core.GenericError, "cannot read %s/%s", s, key, err)
	}
	if res.status == 404 {
		return os.ErrNotExist
	}
	if res.status < 200 || res.status >= 300 {
		return core.Error(core.GenericError, "cannot read %s/%s: status %d, body: %s", s, key, res.status, responseSnippet(res.body))
	}
	if _, err := io.Copy(dest, bytes.NewReader(res.body)); err != nil {
		return core.Error(core.GenericError, "cannot read %s/%s", s, key, err)
	}
	core.End("")
	return nil
}

func (s *S3) Write(name string, source io.ReadSeeker, progress chan int64) error {
	core.Start("name %s", name)
	key := path.Join(s.prefix, name)
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return core.Error(core.GenericError, "cannot seek source for '%s'", key, err)
	}
	body, err := io.ReadAll(source)
	if err != nil {
		return core.Error(core.GenericError, "cannot read source for '%s'", key, err)
	}
	res, err := s.signedFetch("PUT", s.objectURL(key), map[string]string{}, body)
	if err != nil {
		return core.Error(core.GenericError, "cannot write %s/%s", s, key, err)
	}
	if res.status < 200 || res.status >= 300 {
		return core.Error(core.GenericError, "cannot write %s/%s: status %d, body: %s", s, key, res.status, responseSnippet(res.body))
	}
	core.End("")
	return nil
}

func (s *S3) ReadDir(dir string, f Filter) ([]fs.FileInfo, error) {
	core.Start("dir %s, filter %+v", dir, f)
	dir = path.Join(s.prefix, dir)

	var prefix string
	if f.Prefix != "" {
		prefix = path.Join(dir, f.Prefix)
	} else if dir == "" {
		prefix = dir
	} else {
		prefix = dir + "/"
	}

	q := url.Values{}
	q.Set("list-type", "2")
	q.Set("delimiter", "/")
	q.Set("prefix", prefix)
	if f.AfterName != "" {
		q.Set("start-after", f.AfterName)
	}
	if f.Suffix == "" && f.MaxResults != 0 {
		q.Set("max-keys", strconv.FormatInt(f.MaxResults, 10))
	}

	u := fmt.Sprintf("%s/%s?%s", s.endpoint, url.PathEscape(s.bucket), q.Encode())
	res, err := s.signedFetch("GET", u, map[string]string{}, nil)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot list %s/%s", s, dir, err)
	}
	if res.status < 200 || res.status >= 300 {
		return nil, core.Error(core.GenericError, "cannot list %s/%s: status %d, body: %s", s, dir, res.status, responseSnippet(res.body))
	}

	var parsed s3ListV2Result
	if err := xml.Unmarshal(res.body, &parsed); err != nil {
		return nil, core.Error(core.ParseError, "cannot parse list response for %s/%s", s, dir, err)
	}

	infos := make([]fs.FileInfo, 0)
	var cnt int64

	if !f.OnlyFiles {
		for _, item := range parsed.CommonPrefixes {
			if f.MaxResults != 0 && cnt >= f.MaxResults {
				break
			}
			name := strings.TrimPrefix(item.Prefix, dir)
			name = strings.TrimPrefix(name, "/")
			name = strings.TrimRight(name, "/")
			if name == "" {
				continue
			}
			info := simpleFileInfo{name: name, isDir: true}
			if matchFilter(info, f) {
				infos = append(infos, info)
				cnt++
			}
		}
	}

	if !f.OnlyFolders {
		for _, item := range parsed.Contents {
			if f.MaxResults != 0 && cnt >= f.MaxResults {
				break
			}
			name := strings.TrimPrefix(item.Key, dir)
			name = strings.TrimPrefix(name, "/")
			if name == "" || strings.HasSuffix(name, "/") {
				continue
			}
			modTime := time.Time{}
			if item.LastModified != "" {
				if t, err := time.Parse(time.RFC3339, item.LastModified); err == nil {
					modTime = t
				}
			}
			info := simpleFileInfo{name: name, size: item.Size, isDir: false, modTime: modTime}
			if matchFilter(info, f) {
				infos = append(infos, info)
				cnt++
			}
		}
	}

	core.End("%d files", len(infos))
	return infos, nil
}

func (s *S3) Stat(name string) (fs.FileInfo, error) {
	core.Start("name %s", name)
	key := path.Join(s.prefix, name)
	res, err := s.signedFetch("HEAD", s.objectURL(key), map[string]string{}, nil)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot stat %s/%s", s, key, err)
	}
	if res.status == 404 {
		return nil, os.ErrNotExist
	}
	if res.status < 200 || res.status >= 300 {
		return nil, core.Error(core.GenericError, "cannot stat %s/%s: status %d, body: %s", s, key, res.status, responseSnippet(res.body))
	}
	size := int64(0)
	if raw := res.headers.Get("content-length"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			size = n
		}
	}
	modTime := time.Time{}
	if raw := res.headers.Get("last-modified"); raw != "" {
		if t, err := time.Parse(time.RFC1123, raw); err == nil {
			modTime = t
		}
	}
	core.End("")
	return simpleFileInfo{name: path.Base(key), size: size, isDir: strings.HasSuffix(key, "/"), modTime: modTime}, nil
}

func (s *S3) Delete(name string) error {
	core.Start("name %s", name)
	key := path.Join(s.prefix, name)
	res, err := s.signedFetch("DELETE", s.objectURL(key), map[string]string{}, nil)
	if err != nil {
		return core.Error(core.DbError, "cannot delete %s", key, err)
	}
	if res.status < 200 || res.status >= 300 {
		return core.Error(core.DbError, "cannot delete %s: status %d, body: %s", key, res.status, responseSnippet(res.body))
	}
	core.End("")
	return nil
}

func (s *S3) Close() error {
	core.Start("closing S3 store %s", s.id)
	core.End("")
	return nil
}

func (s *S3) String() string { return s.id }

func (s *S3) Describe() Description {
	return Description{ReadCost: 0.0000004, WriteCost: 0.000005}
}

func (s *S3) objectURL(key string) string {
	bucket := strings.Trim(s.bucket, "/")
	if key == "" {
		return fmt.Sprintf("%s/%s", s.endpoint, url.PathEscape(bucket))
	}
	parts := strings.Split(strings.Trim(key, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return fmt.Sprintf("%s/%s/%s", s.endpoint, url.PathEscape(bucket), strings.Join(parts, "/"))
}

type fetchResponse struct {
	status  int
	headers http.Header
	body    []byte
}

func (s *S3) signedFetch(method, rawURL string, headers map[string]string, body []byte) (*fetchResponse, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	h := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(h[:])
	req.Header.Set("x-amz-content-sha256", payloadHash)

	fetchHeaders := js.Global().Get("Headers").New()
	requestURL := rawURL

	if s.proxy != "" {
		if err := s.signer.SignHTTP(
			context.Background(),
			s.creds,
			req,
			payloadHash,
			"s3",
			s.region,
			time.Now().UTC(),
		); err != nil {
			return nil, err
		}
		for k, vals := range req.Header {
			for _, v := range vals {
				fetchHeaders.Call("append", k, v)
			}
		}
		requestURL = s.proxyURL(rawURL)
	} else {
		// S3 presigned query auth requires X-Amz-Expires to be explicitly set.
		q := req.URL.Query()
		if q.Get("X-Amz-Expires") == "" {
			q.Set("X-Amz-Expires", "900")
			req.URL.RawQuery = q.Encode()
		}
		presignedURL, signedHeaders, err := s.signer.PresignHTTP(
			context.Background(),
			s.creds,
			req,
			payloadHash,
			"s3",
			s.region,
			time.Now().UTC(),
		)
		if err != nil {
			return nil, err
		}
		_ = signedHeaders
		for k, vals := range req.Header {
			if strings.EqualFold(k, "authorization") ||
				strings.EqualFold(k, "x-amz-date") ||
				strings.EqualFold(k, "x-amz-security-token") ||
				strings.EqualFold(k, "x-amz-content-sha256") {
				continue
			}
			for _, v := range vals {
				fetchHeaders.Call("append", k, v)
			}
		}
		requestURL = presignedURL
	}

	init := map[string]any{
		"method":  method,
		"headers": fetchHeaders,
	}
	if method != http.MethodGet && method != http.MethodHead && len(body) > 0 {
		arr := js.Global().Get("Uint8Array").New(len(body))
		js.CopyBytesToJS(arr, body)
		init["body"] = arr
	}
	return jsFetch(requestURL, init)
}

func (s *S3) proxyURL(target string) string {
	sep := "?"
	if strings.Contains(s.proxy, "?") {
		sep = "&"
	}
	return s.proxy + sep + "urlb64=" + base64.RawURLEncoding.EncodeToString([]byte(target))
}

func jsFetch(rawURL string, init map[string]any) (*fetchResponse, error) {
	fetchFn := js.Global().Get("fetch")
	if !fetchFn.Truthy() {
		return nil, core.Error(core.NetError, "window.fetch is not available")
	}
	p := fetchFn.Invoke(rawURL, init)
	respVal, err := awaitPromise(p)
	if err != nil {
		return nil, err
	}

	status := respVal.Get("status").Int()
	headers := http.Header{}
	headersVal := respVal.Get("headers")
	if headersVal.Truthy() {
		iter := headersVal.Call("entries")
		for {
			n := iter.Call("next")
			if n.Get("done").Bool() {
				break
			}
			pair := n.Get("value")
			if pair.Length() >= 2 {
				headers.Add(strings.ToLower(pair.Index(0).String()), pair.Index(1).String())
			}
		}
	}

	var body []byte
	if status != 204 {
		bufPromise := respVal.Call("arrayBuffer")
		bufVal, err := awaitPromise(bufPromise)
		if err != nil {
			return nil, err
		}
		u8 := js.Global().Get("Uint8Array").New(bufVal)
		body = make([]byte, u8.Get("length").Int())
		js.CopyBytesToGo(body, u8)
	}

	return &fetchResponse{status: status, headers: headers, body: body}, nil
}

func awaitPromise(p js.Value) (js.Value, error) {
	if !p.Truthy() {
		return js.Value{}, core.Error(core.GenericError, "invalid promise")
	}
	resultCh := make(chan js.Value, 1)
	errCh := make(chan error, 1)

	var thenFn js.Func
	var catchFn js.Func
	thenFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			resultCh <- args[0]
		} else {
			resultCh <- js.Undefined()
		}
		return nil
	})
	catchFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			errCh <- fmt.Errorf(args[0].String())
		} else {
			errCh <- fmt.Errorf("promise rejected")
		}
		return nil
	})

	p.Call("then", thenFn).Call("catch", catchFn)

	select {
	case v := <-resultCh:
		thenFn.Release()
		catchFn.Release()
		return v, nil
	case err := <-errCh:
		thenFn.Release()
		catchFn.Release()
		return js.Value{}, err
	}
}

func responseSnippet(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}
	const max = 240
	s := string(body)
	if !utf8.ValidString(s) {
		s = string(bytes.Runes(body))
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "<non-text body>"
	}
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
