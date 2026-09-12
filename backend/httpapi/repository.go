package httpapi

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"opskeeper/backend/authorization"
	repositorysvc "opskeeper/backend/repository"
)

type repositoryBundleService interface {
	UploadBundle(ctx context.Context, id string, r io.Reader, size int64) (repositorysvc.UploadResult, error)
}

func registerRepositoryRoutes(router chi.Router, service repositoryBundleService, require func(authorization.Permission) func(http.Handler) http.Handler) {
	if service == nil {
		return
	}
	h := func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
		if err := r.ParseMultipartForm(512 << 20); err != nil {
			writeError(w, r, 400, "invalid_upload", err.Error())
			return
		}
		f, head, err := r.FormFile("bundle")
		if err != nil {
			writeError(w, r, 400, "bundle_required", "multipart field bundle is required")
			return
		}
		defer f.Close()
		size := head.Size
		if size <= 0 {
			writeError(w, r, 400, "bundle_required", "bundle must not be empty")
			return
		}
		out, err := service.UploadBundle(r.Context(), chi.URLParam(r, "resourceID"), f, size)
		if err != nil {
			writeError(w, r, 400, "bundle_upload_failed", fmt.Sprint(err))
			return
		}
		writeJSON(w, 201, out)
	}
	if require != nil {
		router.With(require(authorization.ResourceUpdate)).Post("/resources/{resourceID}/bundle", h)
	} else {
		router.Post("/resources/{resourceID}/bundle", h)
	}
}
