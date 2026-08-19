package identity

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusLocked   = "locked"
)

type User struct {
	ID                 string    `json:"id"`
	Username           string    `json:"username"`
	Email              string    `json:"email"`
	Phone              string    `json:"phone"`
	DisplayName        string    `json:"display_name"`
	Status             string    `json:"status"`
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type SessionMetadata struct {
	UserAgent string
	ClientIP  string
	RequestID string
}

type SessionTokens struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type BootstrapInput struct {
	Username    string
	Email       string
	Phone       string
	DisplayName string
	Password    string
}

type UpdateUserInput struct {
	DisplayName *string
	Email       *string
	Phone       *string
	Status      *string
}

type Preferences struct {
	Theme            string     `json:"theme"`
	SidebarMode      string     `json:"sidebar_mode"`
	SidebarCollapsed bool       `json:"sidebar_collapsed"`
	AvatarUpdatedAt  *time.Time `json:"avatar_updated_at,omitempty"`
}

type UpdatePreferencesInput struct {
	Theme            string
	SidebarMode      string
	SidebarCollapsed bool
}

type CreateUserInput struct {
	Username    string
	Email       string
	Phone       string
	DisplayName string
	Password    string
}

type CreateUserResult struct {
	User            User
	OneTimePassword string
}
