package msg

// FolderList is the ordered list of user-defined folder names for organizing
// sessions in the sidebar. Folder assignment per session lives on
// [ManagedSession.FolderName] so it travels with the session row.
type FolderList struct {
	FolderOrder []string `json:"folder_order"`
}

// CreateFolderRequest is the body for POST /folders.
type CreateFolderRequest struct {
	Name string `json:"name"`
}

// RenameFolderRequest is the body for PUT /folders/{name}.
type RenameFolderRequest struct {
	NewName string `json:"new_name"`
}

// SetSessionFolderRequest is the body for PUT /sessions/{id}/folder.
// An empty Folder string clears the assignment (un-files the session).
type SetSessionFolderRequest struct {
	Folder string `json:"folder"`
}

// FileInactiveRequest is the body for POST /admin/file-inactive.
//
// Sessions whose updated_at is older than InactiveDays and which are not
// already filed move into Folder. Folder is auto-created if it doesn't
// exist in the registry.
type FileInactiveRequest struct {
	InactiveDays int    `json:"inactive_days"`
	Folder       string `json:"folder"`
}

// FileInactiveResponse reports the outcome of a file-inactive sweep.
type FileInactiveResponse struct {
	Moved  int      `json:"moved"`
	Folder string   `json:"folder"`
	IDs    []string `json:"ids,omitempty"`
}

// ArchiveOldRequest is the body for POST /admin/archive-old.
//
// Every session (in any folder other than the Archive folder) whose
// updated_at is older than InactiveDays and which is not currently
// running moves into the Archive folder. The Archive folder is
// auto-created if it doesn't exist in the registry.
type ArchiveOldRequest struct {
	InactiveDays int `json:"inactive_days"`
}

// ArchiveOldResponse reports the outcome of an archive-old sweep.
type ArchiveOldResponse struct {
	Moved int      `json:"moved"`
	IDs   []string `json:"ids,omitempty"`
}

// MarkSessionDoneRequest is the body for POST /sessions/{id}/mark-done.
//
// Done=true atomically (a) emits an EventSessionState{State: completed}
// through the central derivation pipeline so the row state and SSE feed
// agree, and (b) moves the session into the canonical Archive folder.
// Done=false reverses both: state goes to idle, folder is cleared.
//
// If new events flow in after the manual mark, derivation overwrites the
// state (e.g. a new tool_call flips back to tool_running). That's
// intentional — the session re-engaging visibly says "no longer done".
type MarkSessionDoneRequest struct {
	Done bool `json:"done"`
}
