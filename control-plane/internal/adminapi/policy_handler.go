package adminapi

import (
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/adminpolicy"
	"github.com/AirSodaz/gantry/internal/identity"
)

func (h Handler) policyCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	policyID, operation, ok := strings.Cut(r.PathValue("policyID"), ":")
	if !ok || policyID == "" {
		http.NotFound(w, r)
		return
	}
	r.SetPathValue("policyID", policyID)
	switch operation {
	case "validate":
		h.validatePolicy(w, r, actor)
	case "simulate":
		h.simulatePolicy(w, r, actor)
	case "retire":
		h.retirePolicy(w, r, actor)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) policyBindingCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	bindingID, operation, ok := strings.Cut(r.PathValue("bindingID"), ":")
	if !ok || bindingID == "" || operation != "revoke" {
		http.NotFound(w, r)
		return
	}
	r.SetPathValue("bindingID", bindingID)
	h.revokePolicyBinding(w, r, actor)
}

func (h Handler) listPolicies(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	limit, err := queryLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "limit must be a positive integer no greater than 100.")
		return
	}
	result, err := h.policies.List(r.Context(), actor, adminpolicy.ListOptions{Type: r.URL.Query().Get("type"), WorkspaceID: r.URL.Query().Get("workspace_id"), State: r.URL.Query().Get("state"), OwnerID: r.URL.Query().Get("owner_id"), BindingTarget: r.URL.Query().Get("binding_target"), Cursor: r.URL.Query().Get("cursor"), Limit: limit})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h Handler) createPolicy(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminpolicy.CreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	policy, draft, err := h.policies.Create(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"policy": policy, "draft": draft})
}

func (h Handler) getPolicy(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.policies.Get(r.Context(), actor, r.PathValue("policyID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) getPolicyDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.policies.GetDraft(r.Context(), actor, r.PathValue("policyID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+item.ETag+`"`)
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) updatePolicyDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminpolicy.UpdateDraftRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.policies.UpdateDraft(r.Context(), actor, r.PathValue("policyID"), r.Header.Get("If-Match"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+item.ETag+`"`)
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) validatePolicy(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.policies.Validate(r.Context(), actor, r.PathValue("policyID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+item.ETag+`"`)
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) listPolicyVersions(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.policies.ListVersions(r.Context(), actor, r.PathValue("policyID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"next_cursor": nil}})
}

func (h Handler) publishPolicyVersion(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminpolicy.PublishRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.policies.Publish(r.Context(), actor, r.PathValue("policyID"), r.Header.Get("If-Match"), r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handler) listPolicyBindings(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.policies.ListBindings(r.Context(), actor, r.PathValue("policyID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"next_cursor": nil}})
}

func (h Handler) bindPolicy(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminpolicy.BindRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.policies.Bind(r.Context(), actor, r.PathValue("policyID"), r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handler) revokePolicyBinding(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decodeOptionalJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.policies.RevokeBinding(r.Context(), actor, r.PathValue("bindingID"), r.Header.Get("Idempotency-Key"), request.Reason)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) simulatePolicy(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminpolicy.SimulationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.policies.Simulate(r.Context(), actor, r.PathValue("policyID"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) retirePolicy(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decodeOptionalJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.policies.Retire(r.Context(), actor, r.PathValue("policyID"), r.Header.Get("Idempotency-Key"), request.Reason)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
