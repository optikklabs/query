package notifications

import (
	"encoding/json"
)

type CreateChannelRequest struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type UpdateChannelRequest = CreateChannelRequest

type TestChannelResponse struct {
	OK        bool   `json:"ok"`
	ErrorText string `json:"errorText,omitempty"`
}

type CreatePolicyRequest struct {
	Name     string          `json:"name"`
	MatchDSL string          `json:"matchDsl"`
	Actions  json.RawMessage `json:"actions"`
	Enabled  *bool           `json:"enabled,omitempty"`
	Position *int            `json:"position,omitempty"`
}

type UpdatePolicyRequest = CreatePolicyRequest

type CreateTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

type UpdateTemplateRequest = CreateTemplateRequest
