package msg

import "time"

// ──────────────────────────────────────────────────────────────────────────────
// Disk usage
//
// A runner on a Windows machine (in WSL) can measure what takes up space on
// that machine's drives and delete what its owner chooses, so the Machines
// page can show a folder tree with sizes and free a full drive. The runner
// does the work with a Windows helper, llm-bridge-disk-usage.exe, which it
// keeps current as it does the computer-use tool, and keeps each drive's
// latest scan in memory.
//
// The bridge asks with a RunnerDiskUsage carrying a Request and a RequestID
// it mints; the runner answers with the same RequestID and an Answer. A
// deletion runs on after its answer, and the runner reports how it ended
// with a DeletionFinished message carrying the bridge's DeletionID.
//
// Deleting is guarded in three places: the bridge requires the owner to
// type the item's name; the runner refuses a path its latest scan does not
// hold; the Windows helper, which can see the real machine, refuses drive
// roots, Windows itself, the folders Windows and installed programs depend
// on, WSL's own disks, links, and any path that is not what it claims to be.
// ──────────────────────────────────────────────────────────────────────────────

// RunnerMsgDiskUsage carries a disk-usage request (bridge → runner), its
// answer, or a finished deletion (runner → bridge).
const RunnerMsgDiskUsage RunnerMessageType = "disk_usage"

// DiskUsageTool is a runner's disk-usage helper, reported in its Hello. Nil
// in a Hello means the runner cannot measure or delete on its machine.
type DiskUsageTool struct {
	Command string `json:"command"`           // absolute path on the runner's machine
	Version string `json:"version,omitempty"` // the helper's build stamp
}

// RunnerDiskUsage is the payload of a RunnerMsgDiskUsage frame. Exactly one
// of Request, Answer and DeletionFinished is set.
type RunnerDiskUsage struct {
	// RequestID pairs a Request with its Answer. The bridge mints it.
	RequestID        string                     `json:"request_id,omitempty"`
	Request          *DiskUsageRequest          `json:"request,omitempty"`
	Answer           *DiskUsageAnswer           `json:"answer,omitempty"`
	DeletionFinished *DiskUsageDeletionFinished `json:"deletion_finished,omitempty"`
}

// DiskUsageAction is what a DiskUsageRequest asks for.
type DiskUsageAction string

const (
	// DiskUsageActionStatus asks for the drives, the scans and the deletion
	// in progress. It needs no other field.
	DiskUsageActionStatus DiskUsageAction = "status"
	// DiskUsageActionStartScan starts a scan of Drive ("C:"), replacing
	// that drive's previous scan when it finishes.
	DiskUsageActionStartScan DiskUsageAction = "start_scan"
	// DiskUsageActionListChildren asks for the entry at Path in its drive's
	// latest scan and the entries directly inside it.
	DiskUsageActionListChildren DiskUsageAction = "list_children"
	// DiskUsageActionCheckDeletion asks whether the entry at Path may be
	// deleted, and why not, without deleting anything.
	DiskUsageActionCheckDeletion DiskUsageAction = "check_deletion"
	// DiskUsageActionDelete deletes the entry at Path for good (it does not
	// go to the Recycle Bin), after the same checks as CheckDeletion and a
	// check that it is still the ExpectedKind. The answer comes once the
	// deletion has started; DeletionFinished reports how it ended.
	DiskUsageActionDelete DiskUsageAction = "delete"
)

// DiskUsageRequest is a bridge → runner request.
type DiskUsageRequest struct {
	Action       DiskUsageAction    `json:"action"`
	Drive        string             `json:"drive,omitempty"`         // start_scan: "C:"
	Path         string             `json:"path,omitempty"`          // list_children, check_deletion, delete: `C:\Games\Old`; a drive root is `C:\`
	ExpectedKind DiskUsageEntryKind `json:"expected_kind,omitempty"` // delete: what the owner saw
	DeletionID   string             `json:"deletion_id,omitempty"`   // delete: the bridge's record of this deletion
}

// DiskUsageAnswer is a runner → bridge answer. Error is set when the runner
// could not do what was asked; the rest is set by action.
type DiskUsageAnswer struct {
	Error         string                  `json:"error,omitempty"`
	Status        *DiskUsageStatus        `json:"status,omitempty"`         // status, start_scan, delete
	Children      *DiskUsageChildren      `json:"children,omitempty"`       // list_children
	DeletionCheck *DiskUsageDeletionCheck `json:"deletion_check,omitempty"` // check_deletion, and delete when it refused
}

// DiskUsageStatus is what the runner holds now.
type DiskUsageStatus struct {
	Drives   []DiskUsageDrive   `json:"drives"`
	Scans    []DiskUsageScan    `json:"scans"`
	Deletion *DiskUsageDeletion `json:"deletion,omitempty"` // the deletion running now, or the last one since the runner started
}

// DiskUsageDrive is a fixed drive the helper can scan.
type DiskUsageDrive struct {
	Drive      string `json:"drive"` // "C:"
	Label      string `json:"label,omitempty"`
	TotalBytes uint64 `json:"total_bytes"`
	FreeBytes  uint64 `json:"free_bytes"` // free to the runner's Windows user
}

// DiskUsageScanState is where a scan is.
type DiskUsageScanState string

const (
	DiskUsageScanRunning  DiskUsageScanState = "running"
	DiskUsageScanFinished DiskUsageScanState = "finished"
	DiskUsageScanFailed   DiskUsageScanState = "failed"
)

// DiskUsageScan is one drive's latest scan.
type DiskUsageScan struct {
	Drive      string             `json:"drive"`
	State      DiskUsageScanState `json:"state"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt *time.Time         `json:"finished_at,omitempty"`
	// While running: how far it has got.
	ScannedEntryCount int64  `json:"scanned_entry_count"`
	ScannedBytes      uint64 `json:"scanned_bytes"`
	Error             string `json:"error,omitempty"`
	// Set when finished: the drive's root entry, so the page can draw the
	// top of the tree without another request.
	Root *DiskUsageEntry `json:"root,omitempty"`
	// PreviousFinishedAt is when the scan the page can still browse
	// finished, while a newer one runs or after one failed; nil if none.
	PreviousFinishedAt *time.Time `json:"previous_finished_at,omitempty"`
}

// DiskUsageEntryKind is what an entry in a scan is.
type DiskUsageEntryKind string

const (
	DiskUsageEntryDirectory DiskUsageEntryKind = "directory"
	DiskUsageEntryFile      DiskUsageEntryKind = "file"
	// DiskUsageEntryLink is a symbolic link or junction. The scan does not
	// follow it, so it counts no bytes, and it cannot be deleted from here.
	DiskUsageEntryLink DiskUsageEntryKind = "link"
	// DiskUsageEntrySmallerItems stands for the entries of a directory past
	// the largest ones the scan keeps by name. It has no path of its own.
	DiskUsageEntrySmallerItems DiskUsageEntryKind = "smaller_items"
)

// DiskUsageEntry is one file, directory or link in a scan, with the totals
// of everything under it.
type DiskUsageEntry struct {
	Name string             `json:"name"` // `C:\` for a drive root
	Path string             `json:"path"` // empty for smaller_items
	Kind DiskUsageEntryKind `json:"kind"`
	// SizeBytes is the files' length, not the space they take on disk, so
	// a compressed file counts its full length. Files that live only in
	// the cloud (OneDrive placeholders) count nothing, and are counted in
	// CloudOnlyFileCount instead.
	SizeBytes          uint64 `json:"size_bytes"`
	FileCount          int64  `json:"file_count"`
	DirectoryCount     int64  `json:"directory_count"`
	CloudOnlyFileCount int64  `json:"cloud_only_file_count,omitempty"`
	// UnreadableDirectoryCount counts directories under this one the scan
	// was not allowed to read, so its size may be short.
	UnreadableDirectoryCount int64      `json:"unreadable_directory_count,omitempty"`
	ModifiedAt               *time.Time `json:"modified_at,omitempty"`
	ItemCount                int64      `json:"item_count,omitempty"`   // smaller_items: how many entries it stands for
	HasChildren              bool       `json:"has_children,omitempty"` // a directory the scan holds entries for
	// PartlyDeleted is set when a deletion removed some but not all of it.
	PartlyDeleted bool `json:"partly_deleted,omitempty"`
}

// DiskUsageChildren is an entry and the entries directly inside it, the
// largest first.
type DiskUsageChildren struct {
	Drive     string           `json:"drive"`
	ScannedAt time.Time        `json:"scanned_at"`
	Entry     DiskUsageEntry   `json:"entry"`
	Children  []DiskUsageEntry `json:"children"`
}

// DiskUsageDeletionCheck says whether an entry may be deleted.
type DiskUsageDeletionCheck struct {
	Path               string             `json:"path"`
	Kind               DiskUsageEntryKind `json:"kind"`
	SizeBytes          uint64             `json:"size_bytes"` // from the scan
	FileCount          int64              `json:"file_count"`
	DriveUsedBytes     uint64             `json:"drive_used_bytes"`
	Allowed            bool               `json:"allowed"`
	RefusalReason      string             `json:"refusal_reason,omitempty"`
	Warnings           []string           `json:"warnings,omitempty"`
	ConfirmationName   string             `json:"confirmation_name"` // what the owner must type: the entry's name
	ScannedAt          time.Time          `json:"scanned_at"`
}

// DiskUsageDeletionState is where a deletion is.
type DiskUsageDeletionState string

const (
	DiskUsageDeletionRequested DiskUsageDeletionState = "requested" // the bridge recorded it; the runner has not answered
	DiskUsageDeletionRefused   DiskUsageDeletionState = "refused"   // nothing was deleted
	DiskUsageDeletionRunning   DiskUsageDeletionState = "running"
	DiskUsageDeletionCompleted DiskUsageDeletionState = "completed" // everything was deleted
	DiskUsageDeletionPartial   DiskUsageDeletionState = "partial"   // some files could not be deleted
	DiskUsageDeletionFailed    DiskUsageDeletionState = "failed"    // it stopped before deleting what it could
	// DiskUsageDeletionLost: the runner disconnected or restarted before it
	// reported the end, so what was deleted is not known. Rescan to see.
	DiskUsageDeletionLost DiskUsageDeletionState = "lost"
)

// DiskUsageDeletion is a deletion the runner is doing or has done.
type DiskUsageDeletion struct {
	DeletionID        string                 `json:"deletion_id"`
	Path              string                 `json:"path"`
	Kind              DiskUsageEntryKind     `json:"kind"`
	State             DiskUsageDeletionState `json:"state"`
	StartedAt         time.Time              `json:"started_at"`
	FinishedAt        *time.Time             `json:"finished_at,omitempty"`
	ExpectedSizeBytes uint64                 `json:"expected_size_bytes"`
	DeletedBytes      uint64                 `json:"deleted_bytes"`
	DeletedFileCount  int64                  `json:"deleted_file_count"`
	FailedEntryCount  int64                  `json:"failed_entry_count"`
	Errors            []string               `json:"errors,omitempty"` // the first few
	RefusalReason     string                 `json:"refusal_reason,omitempty"`
}

// DiskUsageDeletionFinished reports how a deletion ended.
type DiskUsageDeletionFinished struct {
	Deletion DiskUsageDeletion `json:"deletion"`
}
