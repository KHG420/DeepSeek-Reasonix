package sessionstore

import (
	"bufio"
	"errors"
	"io/fs"
	"path/filepath"

	"reasonix/internal/state/sessionv4"
)

// SkipReason is the typed class of a session an import could not bring over.
// Wire consumers match these codes, never the error text.
type SkipReason string

const (
	SkipTooLarge          SkipReason = "too_large"
	SkipUnreadableFormat  SkipReason = "unreadable_format"
	SkipSchemaUnsupported SkipReason = "schema_unsupported"
	SkipPermission        SkipReason = "permission"
	SkipCorrupt           SkipReason = "corrupt"
	SkipCopyFailed        SkipReason = "copy_failed"
)

// SkippedSession names one source entry an import left untouched. Path is the
// entry in the source; nothing at that path was changed or removed.
type SkippedSession struct {
	Name   string
	Path   string
	Reason SkipReason
}

// classifySkip maps the reader's structured failure to its class. Size is
// checked before damage because an oversized record is reported as both.
func classifySkip(err error) SkipReason {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return SkipPermission
	case errors.Is(err, sessionv4.ErrTooLarge), errors.Is(err, bufio.ErrTooLong):
		return SkipTooLarge
	case errors.Is(err, sessionv4.ErrUnsupported):
		return SkipSchemaUnsupported
	case errors.Is(err, sessionv4.ErrDamaged):
		return SkipCorrupt
	default:
		return SkipCopyFailed
	}
}

func (r *LegacyReport) skip(path string, reason SkipReason) {
	if r != nil {
		r.Skipped = append(r.Skipped, SkippedSession{Name: filepath.Base(path), Path: path, Reason: reason})
	}
}

func (r *LegacyReport) skipErr(path string, err error) {
	r.skip(path, classifySkip(err))
}
