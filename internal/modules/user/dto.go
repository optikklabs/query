package user

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"securePassword123"`
}

type LoginResponse struct {
	AuthContextResponse
	AccessToken string `json:"accessToken"`
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
