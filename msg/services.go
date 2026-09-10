package msg

// ──────────────────────────────────────────────────────────────────────────────
// Service inventory — the services on this host, their health, and the
// databases each one holds open. Backs the bridge's Services page.
// ──────────────────────────────────────────────────────────────────────────────

// ServiceInventoryResponse is what GET /services answers: healthcheck's
// status for every service it watches, joined to the SQLite files that
// service's processes hold open right now.
//
// Health is read from healthcheck, never measured here; the process and file
// facts are read from /proc at request time. CheckedAt is healthcheck's own
// timestamp, so a stale one says healthcheck has stopped, not that the
// services have.
type ServiceInventoryResponse struct {
	CheckedAt      string                  `json:"checked_at"`
	HealthcheckURL string                  `json:"healthcheck_url"`
	Services       []ServiceInventoryEntry `json:"services"`
}

// ServiceInventoryEntry is one watched service.
//
// Type is healthcheck's check type: "systemd" (Unit names the unit, SystemUnit
// says whether it is a system rather than a user unit), "http" (URL is what it
// probes) or "command" (a guard script; it has no process of its own and lists
// no databases).
//
// PIDs are the processes found for the unit's cgroup or the URL's listening
// port. ProcessLookupError is set when that lookup itself failed — distinct
// from an empty PIDs, which means the lookup worked and found nothing (the
// service is down, or checked by a command).
type ServiceInventoryEntry struct {
	Name               string            `json:"name"`
	Type               string            `json:"type"`
	Status             string            `json:"status"`
	ResponseMs         int64             `json:"response_ms"`
	LastCheck          string            `json:"last_check"`
	LastError          string            `json:"last_error,omitempty"`
	UptimePct24h       float64           `json:"uptime_pct_24h"`
	EnabledState       string            `json:"enabled_state,omitempty"`
	Unit               string            `json:"unit,omitempty"`
	SystemUnit         bool              `json:"system_unit,omitempty"`
	URL                string            `json:"url,omitempty"`
	PIDs               []int             `json:"pids"`
	ProcessLookupError string            `json:"process_lookup_error,omitempty"`
	Databases          []ServiceDatabase `json:"databases"`
}

// ServiceDatabase is one SQLite file a service's process holds open.
//
// SharedWith names the other services in the same inventory holding the same
// file — model-store's store.db is open in model-store, inber-server and
// llm-bridge at once, and a reader deciding who owns a table needs to know.
type ServiceDatabase struct {
	Path       string   `json:"path"`
	SizeBytes  int64    `json:"size_bytes"`
	ModifiedAt string   `json:"modified_at"`
	SharedWith []string `json:"shared_with"`
}

// DatabaseColumn is one column of a table, from PRAGMA table_info.
//
// Masked means the column's name says it holds a credential (token, secret,
// password, api key…) and every value read from it is answered as null. The
// rule is on the name alone, so the column is still listed, still filterable
// by null-ness, and still says why its values are missing.
type DatabaseColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	NotNull    bool   `json:"not_null"`
	PrimaryKey bool   `json:"primary_key"`
	Masked     bool   `json:"masked"`
}

// DatabaseTable is one table or view with its DDL and current row count.
//
// Kind is "table", "view" or "virtual" (an FTS index or other virtual table).
// RowCountError is set when COUNT(*) failed — an FTS table whose module this
// binary lacks, say — and RowCount is then meaningless.
type DatabaseTable struct {
	Name          string           `json:"name"`
	Kind          string           `json:"kind"`
	SQL           string           `json:"sql"`
	RowCount      int64            `json:"row_count"`
	RowCountError string           `json:"row_count_error,omitempty"`
	Columns       []DatabaseColumn `json:"columns"`
}

// DatabaseSchemaResponse is what GET /services/databases/schema answers for
// one file: every table and view in sqlite_master, in the order it lists them.
type DatabaseSchemaResponse struct {
	Path   string          `json:"path"`
	Tables []DatabaseTable `json:"tables"`
}

// DatabaseRowFilter is one `filter=column:op:value` query parameter, echoed
// back as it was applied. Op is one of DatabaseRowFilterOps.
type DatabaseRowFilter struct {
	Column string `json:"column"`
	Op     string `json:"op"`
	Value  string `json:"value"`
}

// DatabaseRowFilterOps lists every filter operator the rows endpoint accepts.
// "null" and "not_null" ignore Value.
var DatabaseRowFilterOps = []string{"eq", "ne", "contains", "gt", "gte", "lt", "lte", "null", "not_null"}

// DatabaseRowsResponse is what GET /services/databases/rows answers: up to
// Limit rows of one table, newest first by default.
//
// Rows are positional over Columns. A value is a number, a string, a
// base64-encoded blob, or null — null both for SQL NULL and for a masked
// column, and Columns says which. OrderBy is the column the rows are sorted
// by; "rowid" when the caller named none and the table has one, "" for a view
// or a WITHOUT ROWID table, which then come back in storage order. TotalRows
// is the count after the filters, before the limit.
type DatabaseRowsResponse struct {
	Path       string              `json:"path"`
	Table      string              `json:"table"`
	Columns    []DatabaseColumn    `json:"columns"`
	Rows       [][]any             `json:"rows"`
	TotalRows  int64               `json:"total_rows"`
	Limit      int                 `json:"limit"`
	OrderBy    string              `json:"order_by"`
	Descending bool                `json:"descending"`
	Filters    []DatabaseRowFilter `json:"filters"`
}
