package connector

import (
	"context"
)

type applicationAdapter struct{}

func (applicationAdapter) Kind() string               { return "Application" }
func (applicationAdapter) Capabilities() []Capability { return nil }
func (applicationAdapter) Test(context.Context) error { return nil }

var _ Adapter = applicationAdapter{}
