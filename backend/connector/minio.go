package connector

import (
	"context"
	"errors"

	mt "opskeeper/backend/tool/minio"
)

type minioAdapter struct{ connection mt.ConnectionInput }

func newMinIOAdapter(target Target, _ Limits) (Adapter, error) {
	in := mt.ConnectionInput{Endpoint: configString(target.Resource.Config, "endpoint"), Region: configString(target.Resource.Config, "region"), Secure: configBool(target.Resource.Config, "secure"), TimeoutSeconds: configInt(target.Resource.Config, "timeout_seconds")}
	s := secretFields(target.Secret)
	in.AccessKey = s["access_key"]
	in.SecretKey = s["secret_key"]
	in.SessionToken = s["session_token"]
	if in.Endpoint == "" {
		return nil, connectorError(CategoryConfiguration, "configure MinIO", false, errors.New("endpoint is required"))
	}
	return &minioAdapter{connection: in}, nil
}
func (a *minioAdapter) Kind() string               { return "MinIO" }
func (a *minioAdapter) Capabilities() []Capability { return []Capability{CapabilityMinIOInspect} }
func (a *minioAdapter) Test(ctx context.Context) error {
	_, e := mt.Health(ctx, a.connection)
	return mt.Error("test MinIO connection", e)
}
