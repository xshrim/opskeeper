package aiengine

import (
	"context"
	"errors"

	"opskeeper/backend/authorization"
)

// AuthorizeResourceUse adapts the existing authorization context to the T02
// ContextResolver and PolicyGateway contracts. API handlers normally populate
// this context through authorization middleware; internal workers may provide
// a narrower filter explicitly before resolving resources.
func AuthorizeResourceUse(ctx context.Context, resource ContextResource) error {
	if resource.ID == "" || resource.ScopeID == "" {
		return errors.New("context resource id and scope are required")
	}
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		if !filter.Allows(resource.ScopeID, resource.ID) {
			return authorization.ErrForbidden
		}
		return nil
	}
	if filter, ok := authorization.ScopeFilterFromContext(ctx); ok {
		if !filter.Allows(resource.ScopeID) {
			return authorization.ErrForbidden
		}
	}
	return nil
}
