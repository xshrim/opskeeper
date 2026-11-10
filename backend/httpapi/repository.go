package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"opskeeper/backend/authorization"
	repositorysvc "opskeeper/backend/repository"
)

type repositoryBundleService interface {
	UploadBundle(ctx context.Context, id string, r io.Reader, size int64) (repositorysvc.UploadResult, error)
	Preview(ctx context.Context, id string) (repositorysvc.PreviewResult, error)
	PreviewDraft(ctx context.Context, subtype, url, branch, username, password string) (repositorysvc.PreviewResult, error)
}

type repositoryPreviewRequest struct {
	Subtype  string `json:"subtype"`
	URL      string `json:"url"`
	Branch   string `json:"branch"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func registerRepositoryRoutes(router chi.Router, service repositoryBundleService, require func(authorization.Permission) func(http.Handler) http.Handler, maxBundleBytes int64) {
	if service == nil {
		return
	}
	if maxBundleBytes <= 0 {
		maxBundleBytes = 512 << 20
	}
	h := func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBundleBytes+repositoryBundleMultipartOverhead)
		if err := r.ParseMultipartForm(maxBundleBytes); err != nil {
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
		router.With(require(authorization.ResourceCreate)).Post("/repository/preview", func(w http.ResponseWriter, r *http.Request) {
			var body repositoryPreviewRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
				writeError(w, r, 400, "invalid_repository_preview", err.Error())
				return
			}
			out, err := service.PreviewDraft(r.Context(), body.Subtype, body.URL, body.Branch, body.Username, body.Password)
			if err != nil {
				writeError(w, r, 400, "repository_preview_failed", fmt.Sprint(err))
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		router.With(require(authorization.ResourceUpdate)).Post("/resources/{resourceID}/bundle", h)
		router.With(require(authorization.ResourceRead)).Get("/resources/{resourceID}/repository/tree", func(w http.ResponseWriter, r *http.Request) {
			out, err := service.Preview(r.Context(), chi.URLParam(r, "resourceID"))
			if err != nil {
				writeError(w, r, 400, "repository_preview_failed", fmt.Sprint(err))
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	} else {
		router.Post("/repository/preview", func(w http.ResponseWriter, r *http.Request) {
			var body repositoryPreviewRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
				writeError(w, r, 400, "invalid_repository_preview", err.Error())
				return
			}
			out, err := service.PreviewDraft(r.Context(), body.Subtype, body.URL, body.Branch, body.Username, body.Password)
			if err != nil {
				writeError(w, r, 400, "repository_preview_failed", fmt.Sprint(err))
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
		router.Post("/resources/{resourceID}/bundle", h)
		router.Get("/resources/{resourceID}/repository/tree", func(w http.ResponseWriter, r *http.Request) {
			out, err := service.Preview(r.Context(), chi.URLParam(r, "resourceID"))
			if err != nil {
				writeError(w, r, 400, "repository_preview_failed", fmt.Sprint(err))
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	}
}
