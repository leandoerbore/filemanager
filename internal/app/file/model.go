package file

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
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
	ID    string `json:"id"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Type  string `json:"type"`
	Bytes []byte `json:"file"`
}

type CreateFileDTO struct {
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Size   int64  `json:"size"`
	Reader io.Reader
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

func (d CreateFileDTO) NormalizeName() {
	d.Name = strings.ReplaceAll(d.Name, " ", "_")
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	d.Name, _, _ = transform.String(t, d.Name)
}

func NewFile(dto CreateFileDTO) (*File, error) {
	bytes, err := ioutil.ReadAll(dto.Reader)
	if err != nil {
		return nil, fmt.Errorf("Failed to create file model, err: %w", err)
	}
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("Failed to create file id, err: %w", err)
	}

	// TODO: убрать ID
	return &File{
		ID:    id.String(),
		Name:  dto.Dir + dto.Name,
		Size:  dto.Size,
		Bytes: bytes,
	}, nil
}
