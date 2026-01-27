package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/logging"
	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
)

type S3 struct {
	client *s3.Client
	bucket string
	id     string
	prefix string
}

type S3ConfigAuth struct {
	AccessKeyId     string `json:"accessKeyId" yaml:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey" yaml:"secretAccessKey"`
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

type s3logger struct{}

func (l s3logger) Logf(classification logging.Classification, format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

// OpenS3 create a new S3 storage
func OpenS3(id string, c S3Config) (Store, error) {
	core.Start("config %v", c)

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: c.Endpoint,
		}, nil
	})

	if c.Region == "" {
		c.Region = "auto"
	}

	options := []func(*config.LoadOptions) error{
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.Auth.AccessKeyId,
			c.Auth.SecretAccessKey, "")),
		config.WithRegion(c.Region),
	}
	switch c.Verbose {
	case 1:
		options = append(options,
			config.WithLogger(s3logger{}),
			config.WithClientLogMode(aws.LogRequest|aws.LogResponse),
		)
	case 2:
		options = append(options,
			config.WithLogger(s3logger{}),
			config.WithClientLogMode(aws.LogRequestWithBody|aws.LogResponseWithBody),
		)
	}

	if c.Proxy != "" {
		proxyConfig := http.ProxyURL(&url.URL{Host: c.Proxy})
		httpClient := &http.Client{
			Transport: &http.Transport{
				Proxy: proxyConfig,
			},
		}
		options = append(options, config.WithHTTPClient(httpClient))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)
	if err != nil {
		return nil, core.Errorw("cannot create S3 config for %s: %v", id, err)
	}

	s := &S3{
		client: s3.NewFromConfig(cfg),
		id:     id,
		bucket: c.Bucket,
		prefix: c.Prefix,
	}

	err = s.createBucketIfNeeded()
	if err != nil {
		err = s.mapError(err)
		return nil, core.Errorw("cannot create bucket %s", c.Bucket, err)
	}

	core.End("")
	return s, nil
}

func (s *S3) ID() string {
	return s.id
}

func (s *S3) createBucketIfNeeded() error {
	core.Start("bucket %s", s.bucket)
	_, err := s.client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		core.Info("bucket %s already exists", s.bucket)
		return nil
	}
	core.Info("bucket %s does not exist: err %v, creating...", s.bucket, err)

	_, err = s.client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return core.Errorw("cannot create bucket %s", s.bucket, err)
	}
	core.End("")
	return nil
}

func (s *S3) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	core.Start("name %s, rang %v", name, rang)
	name = path.Join(s.prefix, name)

	rawObject, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &name,
		//		Range:  r,
	})
	if err != nil {
		err = s.mapError(err)
		if os.IsNotExist(err) {
			return err
		}
		return core.Errorw("cannot read %s/%s", s, name, err)
	}

	_, err = io.Copy(dest, rawObject.Body)
	if err != nil {
		return core.Errorw("cannot read %s/%s", s, name, err)
	}

	rawObject.Body.Close()
	core.End("")
	return nil
}

func (s *S3) Write(name string, source io.ReadSeeker, progress chan int64) error {
	core.Start("name %s", name)
	name = path.Join(s.prefix, name)

	size, err := source.Seek(0, io.SeekEnd)
	if err != nil {
		return core.Errorw("cannot seek source for '%s'", name, err)
	}
	source.Seek(0, io.SeekStart)

	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &name,
		Body:          source,
		ContentLength: &size,
	})
	if err != nil {
		err = s.mapError(err)
		if os.IsNotExist(err) {
			return err
		}
		return core.Errorw("cannot write %s/%s", s, name, err)
	}
	core.End("")
	return nil
}

func (s *S3) ReadDir(dir string, f Filter) ([]fs.FileInfo, error) {
	core.Start("dir %s, filter %+v", dir, f)
	var prefix string

	dir = path.Join(s.prefix, dir)

	if f.Prefix != "" {
		prefix = path.Join(dir, f.Prefix)
	} else if dir == "" {
		prefix = dir
	} else {
		prefix = dir + "/"
	}

	input := &s3.ListObjectsV2Input{
		Bucket:     aws.String(s.bucket),
		Prefix:     aws.String(prefix),
		StartAfter: &f.AfterName,
		Delimiter:  aws.String("/"),
	}

	if f.Suffix == "" && f.MaxResults != 0 {
		i := int32(f.MaxResults)
		input.MaxKeys = &i
	}

	result, err := s.client.ListObjectsV2(context.TODO(), input)
	if err != nil {
		logrus.Errorf("cannot list %s/%s: %v", s.String(), dir, err)
		return nil, s.mapError(err)
	}

	var infos []fs.FileInfo
	var cnt int64

	if !f.OnlyFiles {
		for _, item := range result.CommonPrefixes {
			if f.MaxResults != 0 && cnt >= f.MaxResults {
				break
			}
			cut := len(path.Clean(dir))
			name := strings.TrimRight((*item.Prefix)[cut+1:], "/")

			info := simpleFileInfo{
				name:  name,
				isDir: true,
			}
			if matchFilter(info, f) {
				infos = append(infos, info)
				cnt++
			}
		}
	}

	if !f.OnlyFolders {
		for _, item := range result.Contents {
			if f.MaxResults != 0 && cnt >= f.MaxResults {
				break
			}
			cut := len(path.Clean(dir))
			name := (*item.Key)[cut+1:]

			info := simpleFileInfo{
				name:    name,
				size:    *item.Size,
				isDir:   false,
				modTime: *item.LastModified,
			}
			if matchFilter(info, f) {
				infos = append(infos, info)
				cnt++
			}
		}
	}
	core.End("%d files", len(infos))
	return infos, nil
}

func (s *S3) mapError(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return fs.ErrNotExist
		default:
			return err
		}
	} else {
		return err
	}
}

func (s *S3) Stat(name string) (fs.FileInfo, error) {
	core.Start("name %s", name)
	fullName := s.prefix + "/" + name

	feed, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &fullName,
	})
	if err == nil {
		core.End("")
		return simpleFileInfo{
			name:    path.Base(fullName),
			size:    *feed.ContentLength,
			isDir:   strings.HasSuffix(fullName, "/"),
			modTime: *feed.LastModified,
		}, nil
	}
	err = s.mapError(err)
	if os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	// if os.IsNotExist(err) && !strings.HasSuffix(name, "/") { // check if it is a directory
	// 	return s.Stat(name + "/")
	// }
	if err != nil {
		return nil, core.Errorw("cannot stat %s/%s", s, name, err)
	}

	core.End("")
	return nil, err
}

func (s *S3) Rename(old, new string) error {
	core.Start("old %s, new %s", old, new)
	old = path.Join(s.prefix, old)
	new = path.Join(s.prefix, new)

	_, err := s.client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     &s.bucket,
		CopySource: aws.String(url.QueryEscape(old)),
		Key:        aws.String(new),
	})
	if err != nil {
		return s.mapError(err)
	}
	core.End("")
	return nil
}

func (s *S3) Delete(name string) error {
	core.Start("name %s", name)
	name = path.Join(s.prefix, name)

	input := &s3.ListObjectsInput{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(name + "/"),
		//Delimiter: aws.String("/"),
	}

	result, err := s.client.ListObjects(context.TODO(), input)
	if err == nil && len(result.Contents) > 0 {
		for _, item := range result.Contents {
			_, err = s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: &s.bucket,
				Key:    item.Key,
			})
			if err != nil {
				return core.Errorw("cannot delete %s", *item.Key, s.mapError(err))
			}
		}
	} else {
		_, err = s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: &s.bucket,
			Key:    &name,
		})
		if err != nil {
			return core.Errorw("cannot delete %s", name, s.mapError(err))
		}
	}
	core.End("")

	return s.mapError(err)
}

func (s *S3) Close() error {
	core.Start("closing S3 store %s", s.id)
	core.End("")
	return nil
}

func (s *S3) String() string {
	return s.id
}

// Describe implements Store.
func (s *S3) Describe() Description {
	core.Info("S3 store %s description", s.id)
	return Description{
		ReadCost:  0.0000004,
		WriteCost: 0.000005,
	}
}
