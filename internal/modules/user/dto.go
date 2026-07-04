package user

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"securePassword123"`
}

type LoginResponse struct {
	AuthContextResponse
	AccessToken string `json:"accessToken"`
}

type SignupRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
	OrgName  string `json:"org_name" validate:"required"`
}

type SignupResponse struct {
	LoginResponse
	APIKey string `json:"api_key"`
}

type DeviceCodeResponse struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	Interval   int    `json:"interval"`
	ExpiresIn  int    `json:"expires_in"`
}

type DeviceTokenRequest struct {
	DeviceCode string `json:"device_code" validate:"required"`
}

// DeviceTokenResponse always returns 200; the CLI switches on Status
// (authorization_pending|slow_down|expired_token|complete). Session is set
// only when complete.
type DeviceTokenResponse struct {
	Status  string         `json:"status"`
	Session *LoginResponse `json:"session,omitempty"`
}

type DeviceApproveRequest struct {
	UserCode string `json:"user_code" validate:"required"`
}

type CreateTeamRequest struct {
	TeamName    string `json:"team_name" validate:"required"`
	OrgName     string `json:"org_name" validate:"required"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
}

type CreateUserRequest struct {
	Email     string  `json:"email" validate:"required,email"`
	Name      string  `json:"name" validate:"required"`
	Role      string  `json:"role"`
	Password  string  `json:"password" validate:"required"`
	AvatarURL *string `json:"avatarUrl"`
	TeamID    int64   `json:"teamId" validate:"required,gt=0"`
}

type UpdateProfileRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type UpdatePreferencesRequest struct {
	Preferences UserPreferences `json:"preferences"`
}
