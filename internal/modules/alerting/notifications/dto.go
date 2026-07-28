package notifications

import (
	"encoding/json"
)

type CreateChannelRequest struct {
	Type   string          `json:"type" validate:"required"`
	Name   string          `json:"name" validate:"required"`
	Config json.RawMessage `json:"config"`
}

type UpdateChannelRequest = CreateChannelRequest

type TestChannelResponse struct {
	OK        bool   `json:"ok"`
	ErrorText string `json:"errorText,omitempty"`
}

type CreatePolicyRequest struct {
	Name     string          `json:"name" validate:"required"`
	MatchDSL string          `json:"matchDsl" validate:"required"`
	Actions  json.RawMessage `json:"actions"`
	Enabled  *bool           `json:"enabled,omitempty"`
	Position *int            `json:"position,omitempty"`
}

type UpdatePolicyRequest = CreatePolicyRequest

type CreateTemplateRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	Body        string `json:"body" validate:"required"`
}

type UpdateTemplateRequest = CreateTemplateRequest
