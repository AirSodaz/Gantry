package adminintegration

import "encoding/json"

type Integration struct {
	ID               string   `json:"id"`
	OrganizationID   string   `json:"organization_id"`
	Slug             string   `json:"slug"`
	DisplayName      string   `json:"display_name"`
	State            string   `json:"state"`
	OwnerPrincipalID string   `json:"owner_principal_id"`
	Environments     []string `json:"environments"`
}

type Client struct {
	ID                    string   `json:"id"`
	IntegrationID         string   `json:"integration_id"`
	Environment           string   `json:"environment"`
	AuthModes             []string `json:"auth_modes"`
	Audience              string   `json:"audience"`
	Status                string   `json:"status"`
	CredentialFingerprint string   `json:"credential_fingerprint"`
	ExpiresAt             *string  `json:"expires_at"`
}

type Publication struct {
	ID                   string   `json:"id"`
	IntegrationID        string   `json:"integration_id"`
	ClientID             string   `json:"client_id"`
	WorkspaceID          string   `json:"workspace_id"`
	Environment          string   `json:"environment"`
	RevisionHash         string   `json:"revision_hash"`
	InputContractDigest  string   `json:"input_contract_digest"`
	OutputContractDigest string   `json:"output_contract_digest"`
	AuthorityModes       []string `json:"authority_modes"`
	State                string   `json:"state"`
	EffectiveUntil       *string  `json:"effective_until"`
}

type Webhook struct {
	ID                    string          `json:"id"`
	IntegrationID         string          `json:"integration_id"`
	Environment           string          `json:"environment"`
	Destination           string          `json:"destination"`
	Status                string          `json:"status"`
	SigningKeyFingerprint string          `json:"signing_key_fingerprint"`
	SubscribedEvents      []string        `json:"subscribed_events"`
	RetryPolicy           json.RawMessage `json:"retry_policy"`
}

type Delivery struct {
	ID            string  `json:"id"`
	EndpointID    string  `json:"endpoint_id"`
	EventID       string  `json:"event_id"`
	DeliveryID    string  `json:"delivery_id"`
	Attempt       int     `json:"attempt"`
	State         string  `json:"state"`
	ResponseClass *string `json:"response_class"`
	NextAttemptAt *string `json:"next_attempt_at"`
}

type CreateIntegrationRequest struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}
type PatchIntegrationRequest struct {
	DisplayName string `json:"display_name"`
}
type CreateClientRequest struct {
	Environment           string   `json:"environment"`
	AuthModes             []string `json:"auth_modes"`
	Audience              string   `json:"audience"`
	CredentialFingerprint string   `json:"credential_fingerprint"`
	ExpiresAt             *string  `json:"expires_at"`
}
type CreatePublicationRequest struct {
	ClientID             string   `json:"client_id"`
	WorkspaceID          string   `json:"workspace_id"`
	Environment          string   `json:"environment"`
	RevisionHash         string   `json:"revision_hash"`
	InputContractDigest  string   `json:"input_contract_digest"`
	OutputContractDigest string   `json:"output_contract_digest"`
	AuthorityModes       []string `json:"authority_modes"`
	EffectiveUntil       *string  `json:"effective_until"`
}
type CreateWebhookRequest struct {
	Environment           string          `json:"environment"`
	Destination           string          `json:"destination"`
	SigningKeyFingerprint string          `json:"signing_key_fingerprint"`
	SubscribedEvents      []string        `json:"subscribed_events"`
	RetryPolicy           json.RawMessage `json:"retry_policy"`
}
