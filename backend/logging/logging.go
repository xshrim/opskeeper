package logging

import (
	"fmt"
	"io"
	"log/slog"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

func New(output io.Writer, format string) (*slog.Logger, error) {
	var handler slog.Handler
	switch format {
	case FormatText:
		return NewText(output), nil
	case FormatJSON:
		handler = slog.NewJSONHandler(output, nil)
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
	return slog.New(handler), nil
}

func NewText(output io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(output, nil))
}
