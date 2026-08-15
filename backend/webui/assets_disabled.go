//go:build !embed_webui

package webui

import "io/fs"

func embeddedFiles() (fs.FS, bool, error) {
	return nil, false, nil
}
