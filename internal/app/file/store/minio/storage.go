package minio

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

var (
	bucket = "static"
)

type Object struct {
	ID   string
	Size int64
	Tags map[string]string
}

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

// TODO: пофиксит передачу имени файла
func (c *Client) GetFile(ctx context.Context, fileName string) (*minio.Object, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, fileName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *Client) GetFiles(ctx context.Context) ([]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var objects []*minio.Object
	for lobj := range c.client.ListObjects(reqCtx, c.bucket, minio.ListObjectsOptions{WithMetadata: true, Recursive: true}) {
		if lobj.Err != nil {
			c.logger.Errorf("Failed to list object from minio object: %s. err: %v", c.bucket, lobj.Err)
			continue
		}
		object, err := c.client.GetObject(reqCtx, c.bucket, lobj.Key, minio.GetObjectOptions{})
		if err != nil {
			c.logger.Errorf("Failed to get object key=%s from minio bucket: %s. err: %v", lobj.Key, c.bucket, lobj.Err)
			continue
		}

		objects = append(objects, object)
	}

	var files []string
	for _, obj := range objects {
		stat, err := obj.Stat()
		if err != nil {
			c.logger.Errorf("Failed to get objects. err: %v", err)
			continue
		}

		files = append(files, stat.Key)
		obj.Close()
	}

	return files, nil
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
	err := c.client.RemoveObject(ctx, c.bucket, fileName, minio.RemoveObjectOptions{})
	if err != nil {
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
