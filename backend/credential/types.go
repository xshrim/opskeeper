package credential

import "time"

type Credential struct {
	ID         string    `json:"id"`
	ScopeID    string    `json:"scope_id"`
	Name       string    `json:"name"`
	Purpose    string    `json:"purpose"`
	KeyVersion string    `json:"key_version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateInput struct {
	ScopeID string
	Name    string
	Purpose string
	Secret  string
}

type UpdateInput struct {
	Name    *string
	Purpose *string
	Secret  *string
}

type Encryptor interface {
	Encrypt([]byte) ([]byte, string, error)
}

type Decryptor interface {
	Decrypt([]byte, string) ([]byte, error)
}
