package inmem

import (
	"fmt"
	"strings" // Added for strings.Builder

	"github.com/open-policy-agent/opa/v1/storage"
)

const ArrayIndexTypeMsg = "array index must be integer"
const DoesNotExistMsg = "document does not exist"
const OutOfRangeMsg = "array index out of range"

func NewNotFoundError(path storage.Path) *storage.Error {
	return NewNotFoundErrorWithHint(path, DoesNotExistMsg)
}

func NewNotFoundErrorWithHint(path storage.Path, hint string) *storage.Error {
	var sb strings.Builder
	sb.WriteString(path.String())
	sb.WriteString(": ")
	sb.WriteString(hint)
	message := sb.String()
	return &storage.Error{
		Code:    storage.NotFoundErr,
		Message: message,
	}
}

func NewNotFoundErrorf(f string, a ...any) *storage.Error {
	msg := fmt.Sprintf(f, a...)
	return &storage.Error{
		Code:    storage.NotFoundErr,
		Message: msg,
	}
}

func NewWriteConflictError(p storage.Path) *storage.Error {
	return &storage.Error{
		Code:    storage.WriteConflictErr,
		Message: p.String(),
	}
}
