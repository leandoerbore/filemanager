package file

import (
	"mime/multipart"

	"github.com/minio/minio-go/v7"
)

type FileNames struct {
	ID string `json:"id"`
}

type Rename struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type Move struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

type Upload struct {
	Name string
	Size int64
	Type string
	Data multipart.File
}

type File struct {
	ID   string        `json:"id"`
	Size int64         `json:"size"`
	Type string        `json:"type"`
	Obj  *minio.Object `json:"-"`
}

type Dir struct {
	SubDirs []string
	Files   []string
}

type SubDir struct {
	Name    string   `json:"name"`
	SubDirs []SubDir `json:"subDirs"`
	Files   []string `json:"files"`
}
