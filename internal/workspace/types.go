package workspace

import "errors"

// MarkerName is the directory that marks a workspace root.
const MarkerName = ".jianwu"

// ConfigFileName is the workspace config file inside MarkerName.
const ConfigFileName = "config.yaml"

// SchemaVersionFileName is the workspace schema version file.
const SchemaVersionFileName = "schema_version"

// CurrentSchemaVersion is the workspace schema version this build supports.
const CurrentSchemaVersion = "1"

// ErrWorkspaceNotFound is returned when no .jianwu/ is found walking up.
var ErrWorkspaceNotFound = errors.New("workspace not found: no .jianwu/ in this or any parent directory")

// InitOpts controls Init behavior.
type InitOpts struct {
	// Bare: when true, do not create books/exports/archive directories.
	Bare bool
}
