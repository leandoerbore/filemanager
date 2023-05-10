package minio

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	model "filemanager/internal/app/file"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

var (
	bucket = "static"
)

type Client struct {
	logger *logrus.Logger
	client *minio.Client
	bucket string
}

func NewClient(endpoint, accessKey, secretKey string, logger *logrus.Logger) (*Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to create minio client. err: %w", err)
	}

	return &Client{
		logger: logger,
		bucket: bucket,
		client: client,
	}, nil
}

func (c *Client) GetFile(ctx context.Context, fileName string) (*minio.Object, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, fileName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *Client) GetFiles(ctx context.Context) ([]model.SubDir, error) {
	objectCh := c.client.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    "backend",
		Recursive: true,
	})

	var folders []string
	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}

		name := strings.TrimPrefix(object.Key, "backend/")
		name = strings.TrimSuffix(name, "/")

		folders = append(folders, name)
	}

	subDir := toTree(folders)

	if subDir == nil {
		return nil, fmt.Errorf("Empty data")
	}

	return subDir, nil
}

func toTree(objectKeys []string) []model.SubDir {
	dirsMap := make(map[string]model.Dir)

	if len(objectKeys) == 1 {
		key := objectKeys[0]
		if i := strings.IndexByte(key, '/'); i > 0 {
			nameDir := key[:i]
			subPath := key[i+1:]

			var (
				f, sb []string
				err   error
			)
			subPath, err = url.QueryUnescape(subPath)
			if err != nil {

			} else {
				if isFile(subPath) {
					f = []string{subPath}
				} else {
					sb = []string{subPath}
				}
			}

			dirsMap[nameDir] = model.Dir{
				SubDirs: sb,
				Files:   f,
			}
		} else {
			dirsMap[key] = model.Dir{}
		}
	} else {
		for _, key := range objectKeys {
			if i := strings.IndexByte(key, '/'); i > 0 {
				nameDir := key[:i]
				subPath := key[i+1:]
				sb := dirsMap[nameDir].SubDirs
				f := dirsMap[nameDir].Files

				var err error
				subPath, err = url.QueryUnescape(subPath)
				if err != nil {

				} else {
					if isFile(subPath) {
						f = append(f, subPath)
					} else {
						sb = append(sb, subPath)
					}
				}

				dirsMap[nameDir] = model.Dir{
					SubDirs: sb,
					Files:   f,
				}
			} else {
				dirsMap[key] = model.Dir{}
			}
		}
	}

	subDirs := make([]model.SubDir, len(dirsMap))
	i := 0
	for k, v := range dirsMap {
		subDirs[i] = model.SubDir{
			Name:    k,
			SubDirs: toTree(v.SubDirs),
			Files:   v.Files,
		}
		i++
	}

	sort.Slice(subDirs, func(i, j int) bool {
		return subDirs[j].Name > subDirs[i].Name
	})

	return subDirs
}

func isFile(path string) bool {
	if strings.Contains(path, ".") && !strings.Contains(path, "/") {
		return true
	}
	return false
}

func (c *Client) UploadFile(ctx context.Context, fileName string, fileSize int64, reader io.Reader) error {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	exists, errBucketExists := c.client.BucketExists(ctx, c.bucket)
	if errBucketExists != nil || !exists {
		c.logger.Warnf("no bucket %s. creating new one...", c.bucket)
		err := c.client.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("Failed to create new bucket. err: %w", err)
		}
	}

	c.logger.Debugf("put new object %s to bucket %s", fileName, c.bucket)
	_, err := c.client.PutObject(reqCtx, c.bucket, fileName, reader, fileSize,
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		})
	if err != nil {
		return fmt.Errorf("Failed to upload file. err: %w", err)
	}

	return nil
}

func (c *Client) RemoveFile(ctx context.Context, fileName string) error {
	c.logger.Infof("name: %s", fileName)
	if err := c.client.RemoveObject(ctx, c.bucket, fileName, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("Failed to delete file. err: %w", err)
	}

	return nil
}

func (c *Client) RenameFile(ctx context.Context, old, new string) error {
	src := minio.CopySrcOptions{
		Bucket: c.bucket,
		Object: old,
	}

	dst := minio.CopyDestOptions{
		Bucket: c.bucket,
		Object: new,
	}

	_, err := c.client.CopyObject(ctx, dst, src)
	if err != nil {
		return fmt.Errorf("Failed to rename file. err: %w", err)
	}

	if err := c.client.RemoveObject(ctx, c.bucket, old, minio.RemoveObjectOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *Client) CreateDirectory(ctx context.Context, dir string) error {
	if _, err := c.client.PutObject(ctx, c.bucket, dir+"/", nil, 0, minio.PutObjectOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *Client) RenameDirectory(ctx context.Context, old, new string) error {
	objectCh := c.client.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    old,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return object.Err
		}

		if err := c.RenameFile(ctx, object.Key,
			strings.Replace(
				object.Key,
				path.Dir(strings.TrimRight(object.Key, "/")),
				new,
				1,
			),
		); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) RemoveDirectory(ctx context.Context, dir string) error {
	objectsCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objectsCh)

		for object := range c.client.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
			Prefix:    dir,
			Recursive: true,
		}) {
			if object.Err != nil {
				c.logger.Errorf("list of objects error: %v", object.Err.Error())
			} else {
				objectsCh <- object
			}
		}
	}()

	for rErr := range c.client.RemoveObjects(ctx, c.bucket, objectsCh, minio.RemoveObjectsOptions{
		GovernanceBypass: true,
	}) {
		c.logger.Errorf("remove object error %v", rErr.Err.Error())
	}

	return nil
}
