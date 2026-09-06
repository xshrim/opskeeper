package connector

import (
	"context"
	"encoding/json"

	"opskeeper/backend/mcpserver/docker/client"
)

type dockerAdapter struct {
	connection client.ConnectionInput
}

func newDockerAdapter(target Target) Adapter {
	connection := client.ConnectionInput{}
	setDockerConnectionFromMap(&connection, target.Resource.Config)
	var credentialValues map[string]any
	if json.Unmarshal(target.Secret, &credentialValues) == nil {
		setDockerConnectionFromMapIfEmpty(&connection, credentialValues)
	}
	return dockerAdapter{connection: connection}
}

func (dockerAdapter) Kind() string { return "Docker" }

func (dockerAdapter) Capabilities() []Capability {
	return []Capability{CapabilityDockerInfo, CapabilityDockerImages, CapabilityDockerContainers, CapabilityDockerLogs, CapabilityDockerInspect, CapabilityDockerStats}
}

func (adapter dockerAdapter) Test(ctx context.Context) error {
	return client.Ping(ctx, adapter.connection)
}
