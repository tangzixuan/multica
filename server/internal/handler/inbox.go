package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Inbox assignment-filter scopes. See RFC v3 §B. The three "my_*" scopes are
// the only user-selectable ones; "other" / "none" exist server-side as
// fallback buckets for the "no filter" mode.
const (
	inboxScopeMe       = "me"
	inboxScopeMyAgent  = "my_agent"
	inboxScopeMySquad  = "my_squad"
	inboxScopeOther    = "other"
	inboxScopeNoneItem = "none"
)

// Operation labels published on `inbox:batch-archived`. The frontend uses
// these to pick the right predicate for precise cache updates (RFC v4 §1).
const (
	inboxBatchOpArchiveAll       = "archive_all"
	inboxBatchOpArchiveRead      = "archive_read"
	inboxBatchOpArchiveCompleted = "archive_completed"
)

var inboxAssignmentScopes = map[string]bool{
	inboxScopeMe:      true,
	inboxScopeMyAgent: true,
	inboxScopeMySquad: true,
}

// parseInboxScope reads `?scope=me,my_agent,my_squad` and validates it.
//
// - Missing / unset → ok=true, scopes=nil (= "no filter", all items).
// - Empty string / empty list → 400 (the frontend's "0 chips selected" state
//   must short-circuit before sending the request; reaching here means a
//   contract violation).
// - Non-empty → only `me`/`my_agent`/`my_squad` are accepted. `other` / `none`
//   are server-internal buckets and cannot be requested explicitly.
func parseInboxScope(w http.ResponseWriter, r *http.Request) (scopes []string, ok bool) {
	raw := r.URL.Query().Get("scope")
	if raw == "" {
		if _, present := r.URL.Query()["scope"]; present {
			writeError(w, http.StatusBadRequest, "scope cannot be empty")
			return nil, false
		}
		return nil, true
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !inboxAssignmentScopes[p] {
			writeError(w, http.StatusBadRequest, "invalid scope: "+p)
			return nil, false
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		writeError(w, http.StatusBadRequest, "scope cannot be empty")
		return nil, false
	}
	return out, true
}

type InboxItemResponse struct {
	ID                string          `json:"id"`
	WorkspaceID       string          `json:"workspace_id"`
	RecipientType     string          `json:"recipient_type"`
	RecipientID       string          `json:"recipient_id"`
	Type              string          `json:"type"`
	Severity          string          `json:"severity"`
	IssueID           *string         `json:"issue_id"`
	Title             string          `json:"title"`
	Body              *string         `json:"body"`
	Read              bool            `json:"read"`
	Archived          bool            `json:"archived"`
	CreatedAt         string          `json:"created_at"`
	IssueStatus       *string         `json:"issue_status"`
	IssueAssigneeType *string         `json:"issue_assignee_type"`
	IssueAssigneeID   *string         `json:"issue_assignee_id"`
	AssigneeScope     *string         `json:"assignee_scope"`
	ActorType         *string         `json:"actor_type"`
	ActorID           *string         `json:"actor_id"`
	Details           json.RawMessage `json:"details"`
}

func inboxToResponse(i db.InboxItem) InboxItemResponse {
	return InboxItemResponse{
		ID:            uuidToString(i.ID),
		WorkspaceID:   uuidToString(i.WorkspaceID),
		RecipientType: i.RecipientType,
		RecipientID:   uuidToString(i.RecipientID),
		Type:          i.Type,
		Severity:      i.Severity,
		IssueID:       uuidToPtr(i.IssueID),
		Title:         i.Title,
		Body:          textToPtr(i.Body),
		Read:          i.Read,
		Archived:      i.Archived,
		CreatedAt:     timestampToString(i.CreatedAt),
		ActorType:     textToPtr(i.ActorType),
		ActorID:       uuidToPtr(i.ActorID),
		Details:       json.RawMessage(i.Details),
	}
}

func inboxRowToResponse(r db.ListInboxItemsRow) InboxItemResponse {
	scope := r.AssigneeScope
	return InboxItemResponse{
		ID:                uuidToString(r.ID),
		WorkspaceID:       uuidToString(r.WorkspaceID),
		RecipientType:     r.RecipientType,
		RecipientID:       uuidToString(r.RecipientID),
		Type:              r.Type,
		Severity:          r.Severity,
		IssueID:           uuidToPtr(r.IssueID),
		Title:             r.Title,
		Body:              textToPtr(r.Body),
		Read:              r.Read,
		Archived:          r.Archived,
		CreatedAt:         timestampToString(r.CreatedAt),
		IssueStatus:       textToPtr(r.IssueStatus),
		IssueAssigneeType: textToPtr(r.IssueAssigneeType),
		IssueAssigneeID:   uuidToPtr(r.IssueAssigneeID),
		AssigneeScope:     &scope,
		ActorType:         textToPtr(r.ActorType),
		ActorID:           uuidToPtr(r.ActorID),
		Details:           json.RawMessage(r.Details),
	}
}

func (h *Handler) enrichInboxResponse(ctx context.Context, resp InboxItemResponse, issueID pgtype.UUID) InboxItemResponse {
	if !issueID.Valid {
		return resp
	}
	issue, err := h.Queries.GetIssue(ctx, issueID)
	if err == nil {
		s := issue.Status
		resp.IssueStatus = &s
	}
	return resp
}

func (h *Handler) ListInbox(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	scopes, ok := parseInboxScope(w, r)
	if !ok {
		return
	}

	items, err := h.Queries.ListInboxItems(r.Context(), db.ListInboxItemsParams{
		WorkspaceID:   wsUUID,
		RecipientType: "member",
		RecipientID:   parseUUID(userID),
		UserID:        parseUUID(userID),
		Scopes:        scopes,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list inbox")
		return
	}

	resp := make([]InboxItemResponse, len(items))
	for i, item := range items {
		resp[i] = inboxRowToResponse(item)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) MarkInboxRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	prev, ok := h.loadInboxItemForUser(w, r, id)
	if !ok {
		return
	}
	item, err := h.Queries.MarkInboxRead(r.Context(), prev.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark read")
		return
	}

	userID := requestUserID(r)
	workspaceID := uuidToString(item.WorkspaceID)
	h.publish(protocol.EventInboxRead, workspaceID, "member", userID, map[string]any{
		"item_id":      uuidToString(item.ID),
		"recipient_id": uuidToString(item.RecipientID),
	})

	resp := h.enrichInboxResponse(r.Context(), inboxToResponse(item), item.IssueID)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ArchiveInboxItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	prev, ok := h.loadInboxItemForUser(w, r, id)
	if !ok {
		return
	}
	item, err := h.Queries.ArchiveInboxItem(r.Context(), prev.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive")
		return
	}

	// Archive all sibling inbox items for the same issue (issue-level archive)
	if item.IssueID.Valid {
		h.Queries.ArchiveInboxByIssue(r.Context(), db.ArchiveInboxByIssueParams{
			WorkspaceID:   item.WorkspaceID,
			RecipientType: item.RecipientType,
			RecipientID:   item.RecipientID,
			IssueID:       item.IssueID,
		})
	}

	userID := requestUserID(r)
	workspaceID := uuidToString(item.WorkspaceID)
	h.publish(protocol.EventInboxArchived, workspaceID, "member", userID, map[string]any{
		"item_id":      uuidToString(item.ID),
		"issue_id":     uuidToPtr(item.IssueID),
		"recipient_id": uuidToString(item.RecipientID),
	})

	resp := h.enrichInboxResponse(r.Context(), inboxToResponse(item), item.IssueID)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CountUnreadInbox(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	count, err := h.Queries.CountUnreadInbox(r.Context(), db.CountUnreadInboxParams{
		WorkspaceID:   wsUUID,
		RecipientType: "member",
		RecipientID:   parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count unread inbox")
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"count": count})
}

// GetInboxScopeCounts returns post-dedup counts per assignee_scope for the
// current user's inbox. Drives chip badge numbers (RFC v3 §B.3).
func (h *Handler) GetInboxScopeCounts(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	rows, err := h.Queries.GetInboxScopeCounts(r.Context(), db.GetInboxScopeCountsParams{
		WorkspaceID: wsUUID,
		UserID:      parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load inbox scope counts")
		return
	}
	counts := map[string]int64{
		inboxScopeMe:       0,
		inboxScopeMyAgent:  0,
		inboxScopeMySquad:  0,
		inboxScopeOther:    0,
		inboxScopeNoneItem: 0,
	}
	for _, row := range rows {
		counts[row.AssigneeScope] = row.Count
	}
	writeJSON(w, http.StatusOK, counts)
}

// GetInboxResourceAvailability returns whether the user owns any agent or is
// involved with any squad — used to drive chip enabled/disabled state
// (RFC v3 §B.2.2). Intentionally decoupled from inbox contents so a user with
// 0 squad notifications today is not classified as "has no squad".
func (h *Handler) GetInboxResourceAvailability(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	row, err := h.Queries.GetInboxResourceAvailability(r.Context(), db.GetInboxResourceAvailabilityParams{
		WorkspaceID: wsUUID,
		UserID:      parseUUID(userID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load inbox resource availability")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"has_my_agent": row.HasMyAgent,
		"has_my_squad": row.HasMySquad,
	})
}

func (h *Handler) MarkAllInboxRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	scopes, ok := parseInboxScope(w, r)
	if !ok {
		return
	}

	count, err := h.Queries.MarkAllInboxRead(r.Context(), db.MarkAllInboxReadParams{
		WorkspaceID: wsUUID,
		UserID:      parseUUID(userID),
		Scopes:      scopes,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark all inbox read")
		return
	}

	slog.Info("inbox: mark all read", append(logger.RequestAttrs(r), "user_id", userID, "count", count, "scope", scopes)...)
	h.publish(protocol.EventInboxBatchRead, workspaceID, "member", userID, map[string]any{
		"recipient_id": userID,
		"count":        count,
		"scope":        scopes,
	})

	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

func (h *Handler) ArchiveAllInbox(w http.ResponseWriter, r *http.Request) {
	h.archiveAllInboxOp(w, r, inboxBatchOpArchiveAll)
}

func (h *Handler) ArchiveAllReadInbox(w http.ResponseWriter, r *http.Request) {
	h.archiveAllInboxOp(w, r, inboxBatchOpArchiveRead)
}

func (h *Handler) ArchiveCompletedInbox(w http.ResponseWriter, r *http.Request) {
	h.archiveAllInboxOp(w, r, inboxBatchOpArchiveCompleted)
}

// archiveAllInboxOp runs the bulk archive variant identified by `operation`
// and emits a single `inbox:batch-archived` event tagged with both the
// operation and the scope filter, so receivers on other devices can update
// their cache without refetching when feasible (RFC v4 §1).
func (h *Handler) archiveAllInboxOp(w http.ResponseWriter, r *http.Request, operation string) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	scopes, ok := parseInboxScope(w, r)
	if !ok {
		return
	}

	var (
		count int64
		err   error
	)
	switch operation {
	case inboxBatchOpArchiveAll:
		count, err = h.Queries.ArchiveAllInbox(r.Context(), db.ArchiveAllInboxParams{
			WorkspaceID: wsUUID,
			UserID:      parseUUID(userID),
			Scopes:      scopes,
		})
	case inboxBatchOpArchiveRead:
		count, err = h.Queries.ArchiveAllReadInbox(r.Context(), db.ArchiveAllReadInboxParams{
			WorkspaceID: wsUUID,
			UserID:      parseUUID(userID),
			Scopes:      scopes,
		})
	case inboxBatchOpArchiveCompleted:
		count, err = h.Queries.ArchiveCompletedInbox(r.Context(), db.ArchiveCompletedInboxParams{
			WorkspaceID: wsUUID,
			UserID:      parseUUID(userID),
			Scopes:      scopes,
		})
	default:
		writeError(w, http.StatusInternalServerError, "unknown inbox batch archive operation")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive inbox")
		return
	}

	slog.Info("inbox: "+operation, append(logger.RequestAttrs(r), "user_id", userID, "count", count, "scope", scopes)...)
	h.publish(protocol.EventInboxBatchArchived, workspaceID, "member", userID, map[string]any{
		"recipient_id": userID,
		"count":        count,
		"operation":    operation,
		"scope":        scopes,
	})

	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}
