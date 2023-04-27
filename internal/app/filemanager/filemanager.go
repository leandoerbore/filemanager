package filemanager

import (
	"filemanager/internal/app/file/store/minio"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

func Start(config *Config, logger *logrus.Logger) error {
	client, err := minio.NewClient(config.Endpoint, config.AccessKey, config.SecretKey, logger)
	if err != nil {
		return fmt.Errorf("Failed to create minio client. err: %w", err)
	}

	srv := newServer(client, logger)

	return http.ListenAndServe(config.BindAddr, srv)
}
