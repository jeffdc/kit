package mull

import (
	"errors"
	"fmt"

	"mull/internal/git"
	"mull/internal/store"
)

// Error codes are a stable contract: agents branch on them, so a code never
// changes meaning. Messages may change.
const (
	CodeUsage              = "usage"               // bad command line: unknown command or flag, wrong arguments
	CodeInvalid            = "invalid_argument"    // a value is malformed or missing
	CodeNotRepo            = "not_a_repo"          // not inside a git working tree
	CodeNotInitialized     = "not_initialized"     // no mull branch (run mull init, or mull sync on a second machine)
	CodeAlreadyInitialized = "already_initialized" // mull init where the branch exists
	CodeNotFound           = "not_found"           // no plan, phase, item, entry, report or guide by that name
	CodeExists             = "already_exists"      // a plan slug or batch ref already used
	CodeState              = "invalid_state"       // the record is in the wrong state for this
	CodeClaimed            = "claimed"             // the item is claimed by another worktree
	CodeSettled            = "already_settled"     // the inbox entry is already settled
	CodeNotAllowed         = "not_allowed"         // the action is not allowed here (kind rules, self-accept)
	CodeCycle              = "dependency_cycle"    // the needs would form a cycle
	CodeScope              = "scope_required"      // say which plan or item this is about
	CodeTourRequired       = "tour_required"       // UI files changed and no tour was given
	CodeDesignRequired     = "design_required"     // done was given no design notes
	CodeNoChecks           = "no_checks"           // no hard checks are configured
	CodeDirty              = "dirty_tree"          // uncommitted or untracked changes
	CodeChecksFailed       = "checks_failed"       // the hard checks failed
	CodeSyncConflict       = "sync_conflict"       // the same records changed here and on the remote
	CodePushRefused        = "push_refused"        // the mull branch holds non-mull paths
	CodeRemote             = "remote_error"        // the remote could not be reached or refused the push
	CodeBatchInvalid       = "batch_invalid"       // one or more batch lines failed; nothing was written
	CodeInvalidJSON        = "invalid_json"        // a batch line is not a valid JSON object
	CodeBusy               = "busy"                // too many concurrent writers; try again
	CodeGit                = "git_error"           // git failed unexpectedly
	CodeRetired            = "retired"             // the command or op was retired; the message names its replacement
	CodeMigrationRequired  = "migration_required"  // the data is from before Model v2: writes wait for mull migrate
	CodeUnsupportedFormat  = "unsupported_format"  // the data is in a newer format than this mull understands
	CodePortInUse          = "port_in_use"         // mull serve's port is taken (another mull serve?)
	CodeInternal           = "internal"            // anything else
)

// Error is a mull error with a stable code.
type Error struct {
	Code  string      `json:"code"`
	Msg   string      `json:"error"`
	Lines []LineError `json:"lines,omitempty"` // batch only: every failed line
}

func (e *Error) Error() string { return e.Msg }

// LineError is one failed batch line.
type LineError struct {
	Line  int    `json:"line"`
	Op    string `json:"op,omitempty"`
	Ref   string `json:"ref,omitempty"`
	Code  string `json:"code"`
	Error string `json:"error"`
}

func errf(code, format string, a ...any) error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// AsError classifies any error into a coded mull error.
func AsError(err error) *Error {
	var me *Error
	if errors.As(err, &me) {
		return me
	}
	var checks *ErrChecksFailed
	var conflict *ErrSyncConflict
	var gerr *git.Error
	switch {
	case errors.As(err, &checks):
		return &Error{Code: CodeChecksFailed, Msg: err.Error()}
	case errors.As(err, &conflict):
		return &Error{Code: CodeSyncConflict, Msg: err.Error()}
	case errors.Is(err, store.ErrNotInitialized):
		return &Error{Code: CodeNotInitialized, Msg: err.Error()}
	case errors.Is(err, store.ErrBusy):
		return &Error{Code: CodeBusy, Msg: err.Error()}
	case errors.Is(err, store.ErrExists):
		return &Error{Code: CodeAlreadyInitialized, Msg: err.Error()}
	case errors.Is(err, git.ErrNotRepo):
		return &Error{Code: CodeNotRepo, Msg: err.Error()}
	case errors.As(err, &gerr):
		return &Error{Code: CodeGit, Msg: err.Error()}
	}
	return &Error{Code: CodeInternal, Msg: err.Error()}
}
