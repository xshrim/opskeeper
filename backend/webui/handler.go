package webui

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const runtimeBaseMarker = `href="./" data-opsk-runtime-base`

type handler struct {
	assets   fs.FS
	basePath string
	index    []byte
}

func New(basePath string) (http.Handler, error) {
	assets, enabled, err := embeddedFiles()
	if err != nil || !enabled {
		return nil, err
	}
	return newHandler(assets, basePath)
}

func newHandler(assets fs.FS, basePath string) (http.Handler, error) {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read embedded index.html: %w", err)
	}
	if !bytes.Contains(index, []byte(runtimeBaseMarker)) {
		return nil, errors.New("embedded index.html does not contain the runtime base marker")
	}
	baseHref := basePath
	if baseHref != "/" {
		baseHref += "/"
	}
	replacement := `href="` + baseHref + `" data-opsk-runtime-base`
	index = bytes.ReplaceAll(index, []byte(runtimeBaseMarker), []byte(replacement))
	return &handler{assets: assets, basePath: basePath, index: index}, nil
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relativePath := strings.TrimPrefix(request.URL.Path, h.basePath)
	relativePath = strings.TrimPrefix(relativePath, "/")
	if relativePath == "" || relativePath == "index.html" {
		h.serveIndex(writer, request)
		return
	}
	if !fs.ValidPath(relativePath) {
		http.NotFound(writer, request)
		return
	}

	info, err := fs.Stat(h.assets, relativePath)
	if err == nil && !info.IsDir() {
		contents, readErr := fs.ReadFile(h.assets, relativePath)
		if readErr != nil {
			http.Error(writer, "Read static asset", http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(relativePath, "assets/") {
			writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			writer.Header().Set("Cache-Control", "no-cache")
		}
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(writer, request, relativePath, time.Time{}, bytes.NewReader(contents))
		return
	}

	if strings.HasPrefix(relativePath, "assets/") || path.Ext(relativePath) != "" {
		http.NotFound(writer, request)
		return
	}
	h.serveIndex(writer, request)
}

func (h *handler) serveIndex(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(writer, request, "index.html", time.Time{}, bytes.NewReader(h.index))
}
