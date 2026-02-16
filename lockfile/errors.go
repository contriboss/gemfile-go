package lockfile

import (
	"errors"
	"fmt"
)

var (
	ErrLockfileNotFound = errors.New("lockfile not found")
	ErrLockfileInvalid  = errors.New("lockfile invalid")
)

type LockfileErrorKind string

const (
	LockfileNotFound   LockfileErrorKind = "not_found"
	LockfileReadError  LockfileErrorKind = "read_error"
	LockfileParseError LockfileErrorKind = "parse_error"
)

type LockfileError struct {
	Kind LockfileErrorKind
	Path string
	Err  error
}

func (e *LockfileError) Error() string {
	if e == nil {
		return ""
	}

	switch e.Kind {
	case LockfileNotFound:
		return fmt.Sprintf("lockfile not found: %s", e.Path)
	case LockfileReadError:
		return fmt.Sprintf("lockfile read error: %s: %v", e.Path, e.Err)
	case LockfileParseError:
		return fmt.Sprintf("lockfile parse error: %s: %v", e.Path, e.Err)
	default:
		if e.Path != "" {
			return fmt.Sprintf("lockfile error: %s: %v", e.Path, e.Err)
		}
		return fmt.Sprintf("lockfile error: %v", e.Err)
	}
}

func (e *LockfileError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *LockfileError) Is(target error) bool {
	if e == nil {
		return false
	}
	switch target {
	case ErrLockfileNotFound:
		return e.Kind == LockfileNotFound
	case ErrLockfileInvalid:
		return e.Kind == LockfileParseError
	default:
		return false
	}
}
