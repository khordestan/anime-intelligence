# Private tracker notes

Verified PostgreSQL accounts can read/replace plain-text notes on an owned tracker entry. Notes belong to the canonical title entry across viewing passes; they are not catalog metadata and never enter feature inference or an external model.

GET /api/v1/tracker/{entryId}/notes returns notes (empty when unset) and updatedAt. PUT to the same route requires {"notes":"text","expectedUpdatedAt":"timestamp from GET"}. Empty text clears the stored value. Notes are limited to 2,000 Unicode code points; bodies are bounded and unknown fields rejected.

Both operations recheck authenticated verified ownership with forced RLS. Foreign and missing entries return the same 404. A changed tracker timestamp returns 409 tracker_notes_conflict without an overwrite. Progress/rating/rewatch changes also conservatively invalidate the timestamp. Two notes saves cannot silently replace each other; a client must reload and intentionally retry. The timestamp is a concurrency condition, not an authentication credential.

The dashboard loads notes only on explicit edit, renders them as plain text, retains drafts after failed/conflicting saves and provides an explicit discard-and-reload action. Cancel clears the draft. Account/card revisions fence late results, and notes are never saved in browser storage.

Existing user_anime.notes storage and grants are used; no migration is needed. Account export includes saved notes in tracker rows. Removing the tracker/account deletes them. A rewatch retains saved notes. JSON demo mode does not expose these routes.
