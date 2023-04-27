package file

import (
	"bytes"
	"context"

	"github.com/sirupsen/logrus"
)

type service struct {
	storage Storage
	logger  *logrus.Logger
}

func NewService(minioStorage Storage, logger *logrus.Logger) (Service, error) {
	return &service{
		storage: minioStorage,
		logger:  logger,
	}, nil
}

type Service interface {
	GetFile(context.Context, string) (*File, error)
	GetFiles(context.Context) ([]string, error)
	UploadFile(context.Context, CreateFileDTO) error
	RemoveFile(context.Context, string) error
	RenameFile(context.Context, Rename) error
	MoveFile(context.Context, Move) error

	CreateDirectory(context.Context, string) error
	RenameDirectory(context.Context, Rename) error
}

func (s *service) GetFile(ctx context.Context, filename string) (*File, error) {
	file, err := s.storage.GetFile(ctx, filename)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (s *service) GetFiles(ctx context.Context) ([]string, error) {
	files, err := s.storage.GetFiles(ctx)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *service) UploadFile(ctx context.Context, dto CreateFileDTO) error {
	dto.NormalizeName()
	file, err := NewFile(dto)
	if err != nil {
		return err
	}

	if err := s.storage.UploadFile(ctx, file.Name, file.Size, bytes.NewBuffer(file.Bytes)); err != nil {
		return err
	}

	return nil
}

func (s *service) RemoveFile(ctx context.Context, fileName string) error {
	if err := s.storage.RemoveFile(ctx, fileName); err != nil {
		return err
	}

	return nil
}

func (s *service) RenameFile(ctx context.Context, fileName Rename) error {
	if err := s.storage.RenameFile(ctx, fileName.Old, fileName.New); err != nil {
		return err
	}

	return nil
}

func (s *service) MoveFile(ctx context.Context, param Move) error {
	if err := s.storage.RenameFile(ctx, param.Src, param.Dst); err != nil {
		return err
	}

	return nil
}

func (s *service) CreateDirectory(ctx context.Context, dir string) error {
	if err := s.storage.CreateDirectory(ctx, dir); err != nil {
		return err
	}

	return nil
}

func (s *service) RenameDirectory(ctx context.Context, dirName Rename) error {
	if err := s.storage.RenameDirectory(ctx, dirName.Old, dirName.New); err != nil {
		return err
	}

	return nil
}
