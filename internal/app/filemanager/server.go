package filemanager

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/sirupsen/logrus"

	storage "filemanager/internal/app/file"

	"filemanager/internal/app/file/store/minio"
)

type server struct {
	router  *mux.Router
	logger  *logrus.Logger
	service storage.Service
}

const (
	prefix string = "backend/"
)

func newServer(client *minio.Client, logger *logrus.Logger) *server {
	service, err := storage.NewService(client, logger)
	if err != nil {
		logger.Fatal(err)
	}
	s := &server{
		router:  mux.NewRouter(),
		logger:  logger,
		service: service,
	}
	s.configureRouter()

	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *server) configureRouter() {
	s.router.Use(
		handlers.CORS(
			handlers.AllowedOrigins([]string{"http://localhost:3000"}),
			handlers.AllowedMethods([]string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}),
			handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type"}),
		))

	staticRouter := s.router.PathPrefix("/static").HandlerFunc(s.handleGetFile())
	staticRouter.HandlerFunc(s.handleGetFile()).Methods("GET", "OPTIONS")

	fileRouter := s.router.PathPrefix("/file").Subrouter()
	fileRouter.HandleFunc("/upload", s.handleUpload()).Methods("POST", "OPTIONS")
	fileRouter.HandleFunc("/remove", s.handleRemoveFile()).Methods("DELETE", "OPTIONS")
	fileRouter.HandleFunc("/rename", s.handleRenameFile()).Methods("POST", "OPTIONS")
	fileRouter.HandleFunc("/move", s.handleMoveFile()).Methods("POST", "OPTIONS")

	dirRouter := s.router.PathPrefix("/dir").Subrouter()
	dirRouter.HandleFunc("/create", s.handleCreateDirectory()).Methods("POST", "OPTIONS")
	dirRouter.HandleFunc("/rename", s.handleRenameDirectory()).Methods("POST", "OPTIONS")
	dirRouter.HandleFunc("/move", s.handleMoveDirectory()).Methods("POST", "OPTIONS")
	dirRouter.HandleFunc("/remove", s.handleRemoveDirectory()).Methods("DELETE", "OPTIONS")
}

func (s *server) handleUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("UPLOAD FILE")
		w.Header().Set("Content-Type", "form/json")

		err := r.ParseMultipartForm(32 << 20)
		if err != nil {
			s.error(w, r, http.StatusBadRequest, err)
			return
		}

		files, ok := r.MultipartForm.File["file"]
		if !ok || len(files) == 0 {
			s.error(w, r, http.StatusBadRequest, err)
			return
		}

		dir, ok := r.MultipartForm.Value["dir"]
		if !ok {
			s.error(w, r, http.StatusBadRequest, err)
			return
		}

		for _, file := range files {
			fileReader, err := file.Open()
			if err != nil {
				s.error(w, r, http.StatusBadRequest, err)
			}

			name := strings.ReplaceAll(file.Filename, " ", "_")
			if err != nil {
				s.error(w, r, http.StatusBadRequest, err)
			}
			name = fmt.Sprintf("%s%s/%s", prefix, dir[0], name)
			fileType := file.Header.Get("Content-Type")

			f := storage.Upload{
				Name: name,
				Type: fileType,
				Size: file.Size,
				Data: fileReader,
			}

			if err := s.service.UploadFile(r.Context(), &f); err != nil {
				s.error(w, r, http.StatusBadRequest, err)
			}
		}

		s.respond(w, r, http.StatusCreated, nil)
	}
}

func (s *server) handleGetFiles() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("Get files from bucket")

		files, err := s.service.GetFiles(r.Context())
		if err != nil {
			s.respond(w, r, http.StatusInternalServerError, nil)
			return
		}

		// TODO: проверить, нужен ли тут маршал
		filesBytes, err := json.Marshal(files)
		if err != nil {
			s.respond(w, r, http.StatusInternalServerError, nil)
			return
		}

		s.respond(w, r, http.StatusOK, filesBytes)
	}
}

func (s *server) handleGetFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := strings.Join(strings.Split(r.URL.Path, "/")[2:], "/")
		if !(fileName == "") {
			file, err := s.service.GetFile(r.Context(), prefix+fileName)
			if err != nil {
				s.error(w, r, http.StatusInternalServerError, err)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(int(file.Size)))
			w.Header().Set("Content-Type", file.Type)
			io.Copy(w, file.Obj)
			s.respond(w, r, http.StatusOK, file)
		} else {
			s.logger.Info("Get files from bucket")

			//files, err := s.service.GetFiles(r.Context())
			files, err := s.service.GetFiles(r.Context())
			if err != nil {
				s.error(w, r, http.StatusInternalServerError, err)
				return
			}

			s.respond(w, r, http.StatusOK, files)
			return
		}

	}
}

func (s *server) handleRemoveFile() http.HandlerFunc {
	type request struct {
		Filename string `json:"filename"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("DELETE FILE")
		w.Header().Set("Content-type", "application/json")

		req := &request{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
			return
		}

		filename := prefix + req.Filename

		if err := s.service.RemoveFile(r.Context(), filename); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
			return
		}

		s.respond(w, r, http.StatusOK, nil)
	}
}

func (s *server) handleRenameFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("RENAME FILE")

		req := &storage.Rename{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
		}
		req.New = prefix + req.New
		req.Old = prefix + req.Old

		if err := s.service.RenameFile(r.Context(), *req); err != nil {
			s.error(w, r, http.StatusInternalServerError, err)
		}

		s.respond(w, r, http.StatusOK, nil)
	}
}

func (s *server) handleMoveFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("MOVE FILE")

		req := &storage.Move{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
			return
		}
		req.Dst = prefix + req.Dst
		req.Src = prefix + req.Src

		if err := s.service.MoveFile(r.Context(), *req); err != nil {
			s.error(w, r, http.StatusInternalServerError, err)
			return
		}

		s.respond(w, r, http.StatusOK, nil)
	}
}

func (s *server) handleCreateDirectory() http.HandlerFunc {
	type request struct {
		Dir string `json:"dir"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("CREATE DIRECTORY")

		req := &request{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
			return
		}

		if err := s.service.CreateDirectory(r.Context(), prefix+req.Dir); err != nil {
			s.error(w, r, http.StatusInternalServerError, err)
			return
		}

		s.respond(w, r, http.StatusCreated, nil)
	}
}

func (s *server) handleRenameDirectory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("RENAME DIRECTORY")

		req := &storage.Rename{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
			return
		}

		req.New = prefix + req.New
		req.Old = prefix + req.Old

		if err := s.service.RenameDirectory(r.Context(), *req); err != nil {
			s.error(w, r, http.StatusInternalServerError, err)
			return
		}

		s.respond(w, r, http.StatusOK, nil)
	}
}

func (s *server) handleMoveDirectory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("MOVE DIRECTORY")

		req := &storage.Move{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
			return
		}

		req.Dst = prefix + req.Dst
		req.Src = prefix + req.Src

		if err := s.service.MoveDirectory(r.Context(), *req); err != nil {
			s.error(w, r, http.StatusInternalServerError, err)
			return
		}

		s.respond(w, r, http.StatusOK, nil)
	}
}

func (s *server) handleRemoveDirectory() http.HandlerFunc {
	type request struct {
		Dir string `json:"dir"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("REMOVE DIRECTORY")

		req := &request{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, r, http.StatusUnprocessableEntity, err)
			return
		}

		if err := s.service.RemoveDirectory(r.Context(), prefix+req.Dir); err != nil {
			s.error(w, r, http.StatusInternalServerError, err)
			return
		}

		s.respond(w, r, http.StatusOK, nil)
	}
}

func (s *server) respond(w http.ResponseWriter, r *http.Request, code int, data interface{}) {
	w.WriteHeader(code)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func (s *server) error(w http.ResponseWriter, r *http.Request, code int, err error) {
	s.respond(w, r, code, map[string]string{"error": err.Error()})
}
