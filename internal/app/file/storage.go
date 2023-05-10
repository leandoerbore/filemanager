package file

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type Client interface {
	GetFile(context.Context, string) (*minio.Object, error)
	UploadFile(context.Context, string, int64, io.Reader) error
	RemoveFile(context.Context, string) error
	RenameFile(context.Context, string, string) error
	GetFiles(context.Context) ([]SubDir, error)

	CreateDirectory(context.Context, string) error
	RenameDirectory(context.Context, string, string) error
	RemoveDirectory(context.Context, string) error
}
