import type { IssueStatus } from "./issue";

export type InboxSeverity = "action_required" | "attention" | "info";

export type InboxItemType =
  | "issue_assigned"
  | "unassigned"
  | "assignee_changed"
  | "status_changed"
  | "priority_changed"
  | "start_date_changed"
  | "due_date_changed"
  | "new_comment"
  | "mentioned"
  | "review_requested"
  | "task_completed"
  | "task_failed"
  | "agent_blocked"
  | "agent_completed"
  | "reaction_added"
  | "quick_create_done"
  | "quick_create_failed";

/**
 * Inbox assignment scope buckets (RFC v3 §B). The three "my_*" values map to
 * the user-selectable chips; "other" and "none" are server-internal fallback
 * buckets that fill the default-no-filter view but cannot be explicitly
 * filtered to.
 */
export type InboxAssigneeScope =
  | "me"
  | "my_agent"
  | "my_squad"
  | "other"
  | "none";

/** User-selectable subset of InboxAssigneeScope (chips). */
export type InboxFilterScope = "me" | "my_agent" | "my_squad";

export interface InboxItem {
  id: string;
  workspace_id: string;
  recipient_type: "member" | "agent";
  recipient_id: string;
  actor_type: "member" | "agent" | "system" | null;
  actor_id: string | null;
  type: InboxItemType;
  severity: InboxSeverity;
  issue_id: string | null;
  title: string;
  body: string | null;
  issue_status: IssueStatus | null;
  read: boolean;
  archived: boolean;
  created_at: string;
  details: Record<string, string> | null;
  // Server-tagged scope of the issue this inbox item references (RFC v3 §A).
  // Optional because older servers may not emit it.
  issue_assignee_type?: "member" | "agent" | "squad" | null;
  issue_assignee_id?: string | null;
  assignee_scope?: InboxAssigneeScope | null;
}

export type InboxScopeCounts = Record<InboxAssigneeScope, number>;

export interface InboxResourceAvailability {
  has_my_agent: boolean;
  has_my_squad: boolean;
}

/**
 * Identifies which bulk-archive endpoint produced an `inbox:batch-archived`
 * WS event. Frontends use this to choose the right predicate when applying a
 * precise cache update (RFC v4 §1).
 */
export type InboxBatchArchiveOperation =
  | "archive_all"
  | "archive_read"
  | "archive_completed";
