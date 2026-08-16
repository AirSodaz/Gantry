package adminintegration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("admin integration resource not found")
	ErrInvalidInput = errors.New("invalid admin integration input")
	ErrInvalidState = errors.New("invalid admin integration state")
)

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) List(ctx context.Context, actor identity.Principal, state, search, environment string) ([]Integration, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, organization_id, slug, display_name, state, owner_principal_id, EXISTS(SELECT 1 FROM gantry.integration_clients c WHERE c.integration_id=i.id AND c.environment='development'), EXISTS(SELECT 1 FROM gantry.integration_clients c WHERE c.integration_id=i.id AND c.environment='staging'), EXISTS(SELECT 1 FROM gantry.integration_clients c WHERE c.integration_id=i.id AND c.environment='production') FROM gantry.integrations i WHERE organization_id=$1 AND ($2='' OR state=$2) AND ($3='' OR slug ILIKE '%' || $3 || '%' OR display_name ILIKE '%' || $3 || '%') AND ($4='' OR EXISTS (SELECT 1 FROM gantry.integration_clients c WHERE c.integration_id=i.id AND c.environment=$4)) ORDER BY display_name,id`, actor.OrganizationID, strings.TrimSpace(state), strings.TrimSpace(search), strings.TrimSpace(environment))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Integration, 0)
	for rows.Next() {
		var item Integration
		var dev, staging, prod bool
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Slug, &item.DisplayName, &item.State, &item.OwnerPrincipalID, &dev, &staging, &prod); err != nil {
			return nil, err
		}
		if dev {
			item.Environments = append(item.Environments, "development")
		}
		if staging {
			item.Environments = append(item.Environments, "staging")
		}
		if prod {
			item.Environments = append(item.Environments, "production")
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, id string) (Integration, error) {
	var item Integration
	var dev, staging, prod bool
	err := s.pool.QueryRow(ctx, `SELECT i.id, i.organization_id, i.slug, i.display_name, i.state, i.owner_principal_id, EXISTS(SELECT 1 FROM gantry.integration_clients c WHERE c.integration_id=i.id AND c.environment='development'), EXISTS(SELECT 1 FROM gantry.integration_clients c WHERE c.integration_id=i.id AND c.environment='staging'), EXISTS(SELECT 1 FROM gantry.integration_clients c WHERE c.integration_id=i.id AND c.environment='production') FROM gantry.integrations i WHERE i.id=$1 AND i.organization_id=$2`, id, actor.OrganizationID).Scan(&item.ID, &item.OrganizationID, &item.Slug, &item.DisplayName, &item.State, &item.OwnerPrincipalID, &dev, &staging, &prod)
	if errors.Is(err, pgx.ErrNoRows) {
		return Integration{}, ErrNotFound
	}
	if err != nil {
		return Integration{}, err
	}
	item.Environments = make([]string, 0, 3)
	if dev {
		item.Environments = append(item.Environments, "development")
	}
	if staging {
		item.Environments = append(item.Environments, "staging")
	}
	if prod {
		item.Environments = append(item.Environments, "production")
	}
	return item, nil
}

func (s *Service) Create(ctx context.Context, actor identity.Principal, req CreateIntegrationRequest) (Integration, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Integration{}, err
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Slug == "" || req.DisplayName == "" || len(req.Slug) > 80 {
		return Integration{}, ErrInvalidInput
	}
	item := Integration{ID: newID("int"), OrganizationID: actor.OrganizationID, Slug: req.Slug, DisplayName: req.DisplayName, State: "active", OwnerPrincipalID: actor.ID}
	item.Environments = []string{}
	err := s.pool.QueryRow(ctx, `INSERT INTO gantry.integrations(id,organization_id,slug,display_name,state,owner_principal_id) VALUES($1,$2,$3,$4,'active',$5) RETURNING id`, item.ID, item.OrganizationID, item.Slug, item.DisplayName, item.OwnerPrincipalID).Scan(&item.ID)
	if err != nil {
		return Integration{}, err
	}
	return item, nil
}

func (s *Service) Patch(ctx context.Context, actor identity.Principal, id string, req PatchIntegrationRequest) (Integration, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Integration{}, err
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		return Integration{}, ErrInvalidInput
	}
	if _, err := s.Get(ctx, actor, id); err != nil {
		return Integration{}, err
	}
	var item Integration
	err := s.pool.QueryRow(ctx, `UPDATE gantry.integrations SET display_name=$3,updated_at=now() WHERE id=$1 AND organization_id=$2 RETURNING id,organization_id,slug,display_name,state,owner_principal_id`, id, actor.OrganizationID, strings.TrimSpace(req.DisplayName)).Scan(&item.ID, &item.OrganizationID, &item.Slug, &item.DisplayName, &item.State, &item.OwnerPrincipalID)
	return item, err
}

func (s *Service) ListClients(ctx context.Context, actor identity.Principal, integrationID string) ([]Client, error) {
	if _, err := s.Get(ctx, actor, integrationID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT c.id,c.integration_id,c.environment,c.auth_modes,c.audience,c.status,c.credential_fingerprint,c.expires_at FROM gantry.integration_clients c JOIN gantry.integrations i ON i.id=c.integration_id WHERE c.integration_id=$1 AND i.organization_id=$2 ORDER BY c.environment,c.id`, integrationID, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Client, 0)
	for rows.Next() {
		var i Client
		var modes []byte
		var t *time.Time
		if err := rows.Scan(&i.ID, &i.IntegrationID, &i.Environment, &modes, &i.Audience, &i.Status, &i.CredentialFingerprint, &t); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(modes, &i.AuthModes)
		if t != nil {
			v := t.UTC().Format(time.RFC3339)
			i.ExpiresAt = &v
		}
		items = append(items, i)
	}
	return items, rows.Err()
}
func (s *Service) CreateClient(ctx context.Context, actor identity.Principal, integrationID string, req CreateClientRequest) (Client, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Client{}, err
	}
	if _, err := s.Get(ctx, actor, integrationID); err != nil {
		return Client{}, err
	}
	if !validEnv(req.Environment) || !validAuthModes(req.AuthModes) || strings.TrimSpace(req.CredentialFingerprint) == "" {
		return Client{}, ErrInvalidInput
	}
	b, _ := json.Marshal(req.AuthModes)
	item := Client{ID: newID("icl"), IntegrationID: integrationID, Environment: req.Environment, AuthModes: req.AuthModes, Audience: strings.TrimSpace(req.Audience), Status: "active", CredentialFingerprint: req.CredentialFingerprint, ExpiresAt: req.ExpiresAt}
	var t *time.Time
	err := s.pool.QueryRow(ctx, `INSERT INTO gantry.integration_clients(id,integration_id,environment,auth_modes,audience,status,credential_fingerprint,expires_at) VALUES($1,$2,$3,$4::jsonb,$5,'active',$6,$7) RETURNING id`, item.ID, integrationID, item.Environment, string(b), item.Audience, item.CredentialFingerprint, parseTime(req.ExpiresAt)).Scan(&item.ID)
	if err != nil {
		return Client{}, err
	}
	_ = t
	return item, nil
}
func (s *Service) RotateClient(ctx context.Context, actor identity.Principal, id string, fingerprint string) (Client, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Client{}, err
	}
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return Client{}, ErrInvalidInput
	}
	var item Client
	var modes []byte
	var t *time.Time
	err := s.pool.QueryRow(ctx, `UPDATE gantry.integration_clients c SET credential_fingerprint=$2,status='active' FROM gantry.integrations i WHERE c.id=$1 AND i.id=c.integration_id AND i.organization_id=$3 RETURNING c.id,c.integration_id,c.environment,c.auth_modes,c.audience,c.status,c.credential_fingerprint,c.expires_at`, id, fingerprint, actor.OrganizationID).Scan(&item.ID, &item.IntegrationID, &item.Environment, &modes, &item.Audience, &item.Status, &item.CredentialFingerprint, &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	if err != nil {
		return Client{}, err
	}
	_ = json.Unmarshal(modes, &item.AuthModes)
	if t != nil {
		v := t.UTC().Format(time.RFC3339)
		item.ExpiresAt = &v
	}
	return item, nil
}
func (s *Service) DisableClient(ctx context.Context, actor identity.Principal, id string) error {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE gantry.integration_clients c SET status='disabled' FROM gantry.integrations i WHERE c.id=$1 AND i.id=c.integration_id AND i.organization_id=$2`, id, actor.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListPublications(ctx context.Context, actor identity.Principal, integrationID string) ([]Publication, error) {
	if _, err := s.Get(ctx, actor, integrationID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.integration_id,p.client_id,p.workspace_id,p.environment,p.revision_hash,p.input_contract_digest,p.output_contract_digest,p.authority_modes,p.state,p.effective_until FROM gantry.integration_publications p JOIN gantry.integrations i ON i.id=p.integration_id WHERE p.integration_id=$1 AND i.organization_id=$2 ORDER BY p.created_at DESC,p.id`, integrationID, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Publication, 0)
	for rows.Next() {
		var i Publication
		var modes []byte
		var t *time.Time
		if err := rows.Scan(&i.ID, &i.IntegrationID, &i.ClientID, &i.WorkspaceID, &i.Environment, &i.RevisionHash, &i.InputContractDigest, &i.OutputContractDigest, &modes, &i.State, &t); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(modes, &i.AuthorityModes)
		if t != nil {
			v := t.UTC().Format(time.RFC3339)
			i.EffectiveUntil = &v
		}
		items = append(items, i)
	}
	return items, rows.Err()
}
func (s *Service) CreatePublication(ctx context.Context, actor identity.Principal, integrationID string, req CreatePublicationRequest) (Publication, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Publication{}, err
	}
	if _, err := s.Get(ctx, actor, integrationID); err != nil {
		return Publication{}, err
	}
	if req.ClientID == "" || req.WorkspaceID == "" || req.RevisionHash == "" || req.InputContractDigest == "" || req.OutputContractDigest == "" || !validEnv(req.Environment) || !validAuthModes(req.AuthorityModes) {
		return Publication{}, ErrInvalidInput
	}
	var clientEnv string
	err := s.pool.QueryRow(ctx, `SELECT environment FROM gantry.integration_clients WHERE id=$1 AND integration_id=$2 AND status='active'`, req.ClientID, integrationID).Scan(&clientEnv)
	if errors.Is(err, pgx.ErrNoRows) {
		return Publication{}, ErrNotFound
	}
	if err != nil {
		return Publication{}, err
	}
	if clientEnv != req.Environment {
		return Publication{}, ErrInvalidInput
	}
	modes, _ := json.Marshal(req.AuthorityModes)
	item := Publication{ID: newID("ipub"), IntegrationID: integrationID, ClientID: req.ClientID, WorkspaceID: req.WorkspaceID, Environment: req.Environment, RevisionHash: req.RevisionHash, InputContractDigest: req.InputContractDigest, OutputContractDigest: req.OutputContractDigest, AuthorityModes: req.AuthorityModes, State: "active", EffectiveUntil: req.EffectiveUntil}
	_, err = s.pool.Exec(ctx, `INSERT INTO gantry.integration_publications(id,integration_id,client_id,workspace_id,environment,revision_hash,input_contract_digest,output_contract_digest,authority_modes,state,effective_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,'active',$10)`, item.ID, integrationID, req.ClientID, req.WorkspaceID, req.Environment, req.RevisionHash, req.InputContractDigest, req.OutputContractDigest, string(modes), parseTime(req.EffectiveUntil))
	return item, err
}
func (s *Service) RevokePublication(ctx context.Context, actor identity.Principal, id string) error {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE gantry.integration_publications p SET state='revoked' FROM gantry.integrations i WHERE p.id=$1 AND i.id=p.integration_id AND i.organization_id=$2 AND p.state='active'`, id, actor.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListWebhooks(ctx context.Context, actor identity.Principal, integrationID string) ([]Webhook, error) {
	if _, err := s.Get(ctx, actor, integrationID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT w.id,w.integration_id,w.environment,w.destination,w.status,w.signing_key_fingerprint,w.subscribed_events,w.retry_policy FROM gantry.webhook_endpoints w JOIN gantry.integrations i ON i.id=w.integration_id WHERE w.integration_id=$1 AND i.organization_id=$2 ORDER BY w.created_at DESC,w.id`, integrationID, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Webhook, 0)
	for rows.Next() {
		var i Webhook
		var events []byte
		if err := rows.Scan(&i.ID, &i.IntegrationID, &i.Environment, &i.Destination, &i.Status, &i.SigningKeyFingerprint, &events, &i.RetryPolicy); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(events, &i.SubscribedEvents)
		items = append(items, i)
	}
	return items, rows.Err()
}
func (s *Service) CreateWebhook(ctx context.Context, actor identity.Principal, integrationID string, req CreateWebhookRequest) (Webhook, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Webhook{}, err
	}
	if _, err := s.Get(ctx, actor, integrationID); err != nil {
		return Webhook{}, err
	}
	if !validEnv(req.Environment) || strings.TrimSpace(req.SigningKeyFingerprint) == "" {
		return Webhook{}, ErrInvalidInput
	}
	u, err := url.Parse(strings.TrimSpace(req.Destination))
	if err != nil || u.Scheme != "https" || u.Host == "" || privateHost(u.Hostname()) {
		return Webhook{}, ErrInvalidInput
	}
	events, _ := json.Marshal(req.SubscribedEvents)
	retry := req.RetryPolicy
	if len(retry) == 0 {
		retry = json.RawMessage(`{}`)
	}
	item := Webhook{ID: newID("whk"), IntegrationID: integrationID, Environment: req.Environment, Destination: u.String(), Status: "active", SigningKeyFingerprint: req.SigningKeyFingerprint, SubscribedEvents: req.SubscribedEvents, RetryPolicy: retry}
	_, err = s.pool.Exec(ctx, `INSERT INTO gantry.webhook_endpoints(id,integration_id,environment,destination,status,signing_key_fingerprint,subscribed_events,retry_policy) VALUES($1,$2,$3,$4,'active',$5,$6::jsonb,$7::jsonb)`, item.ID, integrationID, item.Environment, item.Destination, item.SigningKeyFingerprint, string(events), string(retry))
	return item, err
}
func (s *Service) Redeliver(ctx context.Context, actor identity.Principal, endpointID string, deliveryID string) (Delivery, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Delivery{}, err
	}
	var item Delivery
	var t *time.Time
	err := s.pool.QueryRow(ctx, `INSERT INTO gantry.webhook_deliveries(id,endpoint_id,event_id,delivery_id,attempt,state,response_class,next_attempt_at) SELECT $1,d.endpoint_id,d.event_id,d.delivery_id,d.attempt+1,'queued',NULL,NULL FROM gantry.webhook_deliveries d JOIN gantry.webhook_endpoints w ON w.id=d.endpoint_id JOIN gantry.integrations i ON i.id=w.integration_id WHERE d.endpoint_id=$2 AND d.delivery_id=$3 AND i.organization_id=$4 ORDER BY d.attempt DESC LIMIT 1 RETURNING id,endpoint_id,event_id,delivery_id,attempt,state,response_class,next_attempt_at`, newID("wdel"), endpointID, deliveryID, actor.OrganizationID).Scan(&item.ID, &item.EndpointID, &item.EventID, &item.DeliveryID, &item.Attempt, &item.State, &item.ResponseClass, &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return Delivery{}, ErrNotFound
	}
	if err != nil {
		return Delivery{}, err
	}
	if t != nil {
		v := t.UTC().Format(time.RFC3339)
		item.NextAttemptAt = &v
	}
	return item, nil
}

func validEnv(v string) bool { return v == "development" || v == "staging" || v == "production" }

func validAuthModes(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if value != "application" && value != "delegated_user" {
			return false
		}
	}
	return true
}

func privateHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
}
func parseTime(v *string) any {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *v)
	if err != nil {
		return nil
	}
	return t
}
func newID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}
