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
