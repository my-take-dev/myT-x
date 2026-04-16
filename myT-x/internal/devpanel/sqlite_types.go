package devpanel

// SqliteColumnInfo describes a single SQLite table/view column.
type SqliteColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	NotNull    bool   `json:"not_null"`
	PrimaryKey bool   `json:"primary_key"`
}

// SqliteTableInfo describes a SQLite table or view available for browsing.
type SqliteTableInfo struct {
	Name     string             `json:"name"`
	Columns  []SqliteColumnInfo `json:"columns"`
	RowCount int64              `json:"row_count"`
}

// SqliteQueryResult contains a single page of table rows.
type SqliteQueryResult struct {
	Columns   []string   `json:"columns"`
	Rows      [][]string `json:"rows"`
	NullMasks [][]bool   `json:"null_masks"`
	Offset    int64      `json:"offset"`
	TotalRows int64      `json:"total_rows"`
	Truncated bool       `json:"truncated"`
}

// SqliteExportResult describes a CSV export written inside the session root.
type SqliteExportResult struct {
	Path     string `json:"path"`
	RowCount int64  `json:"row_count"`
}
