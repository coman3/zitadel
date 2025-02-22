package assets

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/caos/logging"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/gorilla/mux"

	"github.com/caos/zitadel/internal/api/authz"
	http_mw "github.com/caos/zitadel/internal/api/http/middleware"
	"github.com/caos/zitadel/internal/command"
	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/id"
	"github.com/caos/zitadel/internal/management/repository"
	"github.com/caos/zitadel/internal/query"
	"github.com/caos/zitadel/internal/static"
)

type Handler struct {
	errorHandler    ErrorHandler
	storage         static.Storage
	commands        *command.Commands
	authInterceptor *http_mw.AuthInterceptor
	idGenerator     id.Generator
	orgRepo         repository.OrgRepository
	query           *query.Queries
}

func (h *Handler) AuthInterceptor() *http_mw.AuthInterceptor {
	return h.authInterceptor
}

func (h *Handler) Commands() *command.Commands {
	return h.commands
}

func (h *Handler) ErrorHandler() ErrorHandler {
	return DefaultErrorHandler
}

func (h *Handler) Storage() static.Storage {
	return h.storage
}

type Uploader interface {
	Callback(ctx context.Context, info *domain.AssetInfo, orgID string, commands *command.Commands) error
	ObjectName(data authz.CtxData) (string, error)
	BucketName(data authz.CtxData) string
	ContentTypeAllowed(contentType string) bool
	MaxFileSize() int64
}

type Downloader interface {
	ObjectName(ctx context.Context, path string) (string, error)
	BucketName(ctx context.Context, id string) string
}

type ErrorHandler func(http.ResponseWriter, *http.Request, error, int)

func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error, code int) {
	logging.Log("ASSET-g5ef1").WithError(err).WithField("uri", r.RequestURI).Error("error occurred on asset api")
	http.Error(w, err.Error(), code)
}

func NewHandler(
	commands *command.Commands,
	verifier *authz.TokenVerifier,
	authConfig authz.Config,
	idGenerator id.Generator,
	storage static.Storage,
	orgRepo repository.OrgRepository,
	queries *query.Queries,
) http.Handler {
	h := &Handler{
		commands:        commands,
		errorHandler:    DefaultErrorHandler,
		authInterceptor: http_mw.AuthorizationInterceptor(verifier, authConfig),
		idGenerator:     idGenerator,
		storage:         storage,
		orgRepo:         orgRepo,
		query:           queries,
	}

	verifier.RegisterServer("Management-API", "assets", AssetsService_AuthMethods) //TODO: separate api?
	router := mux.NewRouter()
	router.Use(sentryhttp.New(sentryhttp.Options{}).Handle)
	RegisterRoutes(router, h)
	router.PathPrefix("/{id}").Methods("GET").HandlerFunc(DownloadHandleFunc(h, h.GetFile()))
	return router
}

func (h *Handler) GetFile() Downloader {
	return &publicFileDownloader{}
}

type publicFileDownloader struct{}

func (l *publicFileDownloader) ObjectName(_ context.Context, path string) (string, error) {
	return path, nil
}

func (l *publicFileDownloader) BucketName(_ context.Context, id string) string {
	return id
}

const maxMemory = 2 << 20
const paramFile = "file"

func UploadHandleFunc(s AssetsService, uploader Uploader) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctxData := authz.GetCtxData(ctx)
		err := r.ParseMultipartForm(maxMemory)
		file, handler, err := r.FormFile(paramFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer func() {
			err = file.Close()
			logging.Log("UPLOAD-GDg34").OnError(err).Warn("could not close file")
		}()
		contentType := handler.Header.Get("content-type")
		size := handler.Size
		if !uploader.ContentTypeAllowed(contentType) {
			s.ErrorHandler()(w, r, fmt.Errorf("invalid content-type: %s", contentType), http.StatusBadRequest)
			return
		}
		if size > uploader.MaxFileSize() {
			s.ErrorHandler()(w, r, fmt.Errorf("file to big, max file size is %vKB", uploader.MaxFileSize()/1024), http.StatusBadRequest)
			return
		}

		bucketName := uploader.BucketName(ctxData)
		objectName, err := uploader.ObjectName(ctxData)
		if err != nil {
			s.ErrorHandler()(w, r, fmt.Errorf("upload failed: %v", err), http.StatusInternalServerError)
			return
		}
		info, err := s.Commands().UploadAsset(ctx, bucketName, objectName, contentType, file, size)
		if err != nil {
			s.ErrorHandler()(w, r, fmt.Errorf("upload failed: %v", err), http.StatusInternalServerError)
			return
		}
		err = uploader.Callback(ctx, info, ctxData.OrgID, s.Commands())
		if err != nil {
			s.ErrorHandler()(w, r, fmt.Errorf("upload failed: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

func DownloadHandleFunc(s AssetsService, downloader Downloader) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Storage() == nil {
			return
		}
		ctx := r.Context()
		id := mux.Vars(r)["id"]
		bucketName := downloader.BucketName(ctx, id)
		path := ""
		if id != "" {
			path = strings.Split(r.RequestURI, id+"/")[1]
		}
		objectName, err := downloader.ObjectName(ctx, path)
		if err != nil {
			s.ErrorHandler()(w, r, fmt.Errorf("download failed: %v", err), http.StatusInternalServerError)
			return
		}
		if objectName == "" {
			s.ErrorHandler()(w, r, fmt.Errorf("file not found: %v", objectName), http.StatusNotFound)
			return
		}
		reader, getInfo, err := s.Storage().GetObject(ctx, bucketName, objectName)
		if err != nil {
			s.ErrorHandler()(w, r, fmt.Errorf("download failed: %v", err), http.StatusInternalServerError)
			return
		}
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			s.ErrorHandler()(w, r, fmt.Errorf("download failed: %v", err), http.StatusInternalServerError)
			return
		}
		info, err := getInfo()
		if err != nil {
			s.ErrorHandler()(w, r, fmt.Errorf("download failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-length", strconv.FormatInt(info.Size, 10))
		w.Header().Set("content-type", info.ContentType)
		w.Header().Set("ETag", info.ETag)
		w.Write(data)
	}
}
