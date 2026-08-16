package credential

import (
	"context"
	"errors"
	"strings"

	"opskeeper/backend/authorization"
)

type Service struct {
	store     Store
	encryptor Encryptor
}

func NewService(store Store, encryptor Encryptor) *Service {
	return &Service{store: store, encryptor: encryptor}
}

func (s *Service) Create(ctx context.Context, actorID string, input CreateInput) (Credential, error) {
	input.ScopeID = strings.TrimSpace(input.ScopeID)
	input.Name = strings.TrimSpace(input.Name)
	input.Purpose = strings.TrimSpace(input.Purpose)
	if err := validateInput(input.ScopeID, input.Name, input.Secret); err != nil {
		return Credential{}, err
	}
	if !allowsExactScope(ctx, input.ScopeID) {
		return Credential{}, authorization.ErrForbidden
	}
	ciphertext, keyVersion, err := s.encryptor.Encrypt([]byte(input.Secret))
	if err != nil {
		return Credential{}, err
	}
	return s.store.Create(ctx, actorID, input, ciphertext, keyVersion)
}

func (s *Service) List(ctx context.Context, actorID string) ([]Credential, error) {
	return s.store.List(ctx, actorID)
}

func (s *Service) Get(ctx context.Context, actorID, id string) (Credential, error) {
	if strings.TrimSpace(id) == "" {
		return Credential{}, invalid("credential_id is required")
	}
	return s.store.Get(ctx, actorID, id)
}

// Reveal is intentionally kept out of HTTP handlers. Internal connectors use
// it only after the caller has passed the normal scope authorization check.
func (s *Service) Reveal(ctx context.Context, actorID, id string) ([]byte, error) {
	if strings.TrimSpace(id) == "" {
		return nil, invalid("credential_id is required")
	}
	ciphertext, keyVersion, scopeID, err := s.store.Secret(ctx, actorID, id)
	if err != nil {
		return nil, err
	}
	if !allowsExactScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	decryptor, ok := s.encryptor.(Decryptor)
	if !ok {
		return nil, errors.New("credential encryptor does not support decryption")
	}
	return decryptor.Decrypt(ciphertext, keyVersion)
}

// RevealLinked decrypts a credential that has already been authorized through
// its owning resource. It is an internal connector API and is never routed to HTTP.
func (s *Service) RevealLinked(ctx context.Context, id string) ([]byte, error) {
	if strings.TrimSpace(id) == "" {
		return nil, invalid("credential_id is required")
	}
	ciphertext, keyVersion, err := s.store.SecretByID(ctx, id)
	if err != nil {
		return nil, err
	}
	decryptor, ok := s.encryptor.(Decryptor)
	if !ok {
		return nil, errors.New("credential encryptor does not support decryption")
	}
	return decryptor.Decrypt(ciphertext, keyVersion)
}

func (s *Service) Update(ctx context.Context, actorID, id string, input UpdateInput) (Credential, error) {
	if strings.TrimSpace(id) == "" {
		return Credential{}, invalid("credential_id is required")
	}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || len([]rune(value)) > 120 {
			return Credential{}, invalid("name must contain 1 to 120 characters")
		}
		input.Name = &value
	}
	if input.Purpose != nil {
		value := strings.TrimSpace(*input.Purpose)
		if len([]rune(value)) > 500 {
			return Credential{}, invalid("purpose must be at most 500 characters")
		}
		input.Purpose = &value
	}
	var ciphertext []byte
	var keyVersion string
	if input.Secret != nil {
		if *input.Secret == "" {
			return Credential{}, invalid("secret must not be empty")
		}
		var err error
		ciphertext, keyVersion, err = s.encryptor.Encrypt([]byte(*input.Secret))
		if err != nil {
			return Credential{}, err
		}
	}
	if input.Name == nil && input.Purpose == nil && input.Secret == nil {
		return Credential{}, invalid("at least one field must be provided")
	}
	current, err := s.store.Get(ctx, actorID, id)
	if err != nil {
		return Credential{}, err
	}
	if !allowsExactScope(ctx, current.ScopeID) {
		return Credential{}, authorization.ErrForbidden
	}
	return s.store.Update(ctx, actorID, id, input, ciphertext, keyVersion)
}

func (s *Service) Delete(ctx context.Context, actorID, id string) error {
	if strings.TrimSpace(id) == "" {
		return invalid("credential_id is required")
	}
	current, err := s.store.Get(ctx, actorID, id)
	if err != nil {
		return err
	}
	if !allowsExactScope(ctx, current.ScopeID) {
		return authorization.ErrForbidden
	}
	return s.store.Delete(ctx, actorID, id)
}

func validateInput(scopeID, name, secret string) error {
	if scopeID == "" {
		return invalid("scope_id is required")
	}
	if name == "" || len([]rune(name)) > 120 {
		return invalid("name must contain 1 to 120 characters")
	}
	if secret == "" {
		return invalid("secret must not be empty")
	}
	return nil
}

func allowsExactScope(ctx context.Context, scopeID string) bool {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	return !restricted || filter.Allows(scopeID)
}
