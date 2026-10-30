package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/DiegoOF07/Diskflow/internal/engine"
)

// Error codes the frontend receives. They are stable: the UI picks its message
// and its state from the code, never from the text. Keep in sync with
// frontend/src/backend/errors.ts.
const (
	CodeNoScan          = "no_scan"
	CodeRootUnavailable = "root_unavailable"
	CodeScanRunning     = "scan_running"
	CodeDBUnavailable   = "db_unavailable"
	CodeAccessDenied    = "access_denied"
	CodeNotFound        = "not_found"
	CodeOther           = "error" // no specific message; the UI shows Detail
)

// Error is what a bound method's promise rejects with in the frontend, as JSON.
type Error struct {
	Code   string `json:"code"`
	Detail string `json:"detail"` // the Go error text, for diagnosis
}

func (e *Error) Error() string { return e.Code + ": " + e.Detail }

// codes maps the engine's sentinel errors to their codes, most specific first.
var codes = []struct {
	err  error
	code string
}{
	{errNotReady, CodeDBUnavailable},
	{engine.ErrNoScan, CodeNoScan},
	{engine.ErrScanRunning, CodeScanRunning},
	{engine.ErrRootUnavailable, CodeRootUnavailable},
	{engine.ErrNotDir, CodeRootUnavailable},
	{engine.ErrRootNotFound, CodeNotFound},
	{engine.ErrNotFound, CodeNotFound},
	{fs.ErrPermission, CodeAccessDenied},
	{fs.ErrNotExist, CodeNotFound},
}

// formatError is the ErrorFormatter: Wails calls it for every error a bound
// method returns. It must return text: the Wails v2 runtime rejects the
// promise with new Error(value), which turns an object into "[object Object]".
func formatError(err error) any {
	b, jerr := json.Marshal(toError(err))
	if jerr != nil {
		return err.Error()
	}
	return string(b)
}

// toError gives err its code.
func toError(err error) *Error {
	for _, c := range codes {
		if errors.Is(err, c.err) {
			return &Error{Code: c.code, Detail: err.Error()}
		}
	}
	return &Error{Code: CodeOther, Detail: err.Error()}
}
