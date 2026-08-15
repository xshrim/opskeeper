//go:build embed_webui

package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedAssets embed.FS

func embeddedFiles() (fs.FS, bool, error) {
	assets, err := fs.Sub(embeddedAssets, "dist")
	return assets, true, err
}
