package gui

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/engine"
	"github.com/thedavidweng/rainforest/internal/store"
)

const (
	ErrValidationFailed     = "VALIDATION_FAILED"
	ErrInvalidFilter        = "INVALID_FILTER"
	ErrAuthRequired         = "AUTH_REQUIRED"
	ErrAuthFailed           = "AUTH_FAILED"
	ErrPermissionDenied     = "PERMISSION_DENIED"
	ErrAPIError             = "API_ERROR"
	ErrRemoteProtocolError  = "REMOTE_PROTOCOL_ERROR"
	ErrResourceNotFound     = "RESOURCE_NOT_FOUND"
	ErrConflict             = "CONFLICT"
	ErrNetworkError         = "NETWORK_ERROR"
	ErrRateLimited          = "RATE_LIMITED"
	ErrPartialSuccess       = "PARTIAL_SUCCESS"
	ErrConfirmationRequired = "CONFIRMATION_REQUIRED"
	ErrReadOnlyViolation    = "READ_ONLY_VIOLATION"
	ErrFilesystemError      = "FILESYSTEM_ERROR"
	ErrConfigError          = "CONFIG_ERROR"
	ErrInterrupted          = "INTERRUPTED"
)

type Error struct {
	Code         string         `json:"code"`
	Category     string         `json:"category"`
	Message      string         `json:"message"`
	RetryAfterMS int64          `json:"retryAfterMs,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	sanitized := Error{
		Code:         e.Code,
		Category:     e.Category,
		Message:      aphotos.Redact(e.Message),
		RetryAfterMS: e.RetryAfterMS,
		Details:      sanitizeDetails(e.Details),
	}
	if sanitized.Code == "" {
		sanitized.Code = ErrValidationFailed
	}
	if sanitized.Category == "" {
		sanitized.Category = CategoryForCode(sanitized.Code)
	}
	b, err := json.Marshal(sanitized)
	if err != nil {
		return `{"code":"` + sanitized.Code + `","category":"` + sanitized.Category + `","message":"internal error"}`
	}
	return string(b)
}

func CategoryForCode(code string) string {
	switch code {
	case ErrValidationFailed, ErrInvalidFilter:
		return "validation"
	case ErrAuthRequired, ErrAuthFailed, ErrPermissionDenied:
		return "auth"
	case ErrAPIError, ErrRemoteProtocolError, ErrResourceNotFound, ErrConflict, ErrPartialSuccess:
		return "api"
	case ErrNetworkError, ErrRateLimited:
		return "network"
	case ErrConfirmationRequired, ErrReadOnlyViolation:
		return "safety"
	case ErrFilesystemError:
		return "filesystem"
	case ErrConfigError:
		return "config"
	case ErrInterrupted:
		return "interrupted"
	default:
		return "validation"
	}
}

func newError(code, message string, details map[string]any) *Error {
	return &Error{
		Code:     code,
		Category: CategoryForCode(code),
		Message:  aphotos.Redact(message),
		Details:  sanitizeDetails(details),
	}
}

func ToError(err error) error {
	return toError(err)
}

func toError(err error) error {
	if err == nil {
		return nil
	}

	var existing *Error
	if errors.As(err, &existing) {
		if existing.Code == "" {
			existing.Code = ErrValidationFailed
		}
		if existing.Category == "" {
			existing.Category = CategoryForCode(existing.Code)
		}
		existing.Message = aphotos.Redact(existing.Message)
		existing.Details = sanitizeDetails(existing.Details)
		return existing
	}

	if errors.Is(err, engine.ErrOffline) || errors.Is(err, credstore.ErrNotConfigured) {
		return newError(ErrAuthRequired, err.Error(), nil)
	}
	if errors.Is(err, store.ErrNotFound) {
		return newError(ErrResourceNotFound, err.Error(), nil)
	}
	if errors.Is(err, context.Canceled) {
		return newError(ErrInterrupted, "operation interrupted", nil)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return newError(ErrNetworkError, err.Error(), nil)
	}

	var cfgErr *aphotos.ConfigError
	if errors.As(err, &cfgErr) {
		var details map[string]any
		if cfgErr.Field != "" {
			details = map[string]any{"field": aphotos.Redact(cfgErr.Field)}
		}
		return newError(ErrConfigError, cfgErr.Error(), details)
	}

	var authErr *aphotos.AuthenticationError
	if errors.As(err, &authErr) {
		return newError(ErrAuthFailed, authErr.Error(), httpDetails(authErr.Endpoint, authErr.StatusCode, authErr.RequestID))
	}

	var permErr *aphotos.PermissionError
	if errors.As(err, &permErr) {
		return newError(ErrPermissionDenied, permErr.Error(), httpDetails(permErr.Endpoint, permErr.StatusCode, permErr.RequestID))
	}

	var rlErr *aphotos.RateLimitError
	if errors.As(err, &rlErr) {
		ge := newError(ErrRateLimited, rlErr.Error(), httpDetails(rlErr.Endpoint, rlErr.StatusCode, rlErr.RequestID))
		if rlErr.RetryAfter > 0 {
			ge.RetryAfterMS = rlErr.RetryAfter.Milliseconds()
		}
		return ge
	}

	var filterErr *aphotos.InvalidFilterError
	if errors.As(err, &filterErr) {
		details := httpDetails(filterErr.Endpoint, filterErr.StatusCode, filterErr.RequestID)
		if filterErr.Filter != "" {
			if details == nil {
				details = map[string]any{}
			}
			details["filter"] = aphotos.Redact(filterErr.Filter)
		}
		return newError(ErrInvalidFilter, filterErr.Error(), details)
	}

	var nfErr *aphotos.NotFoundError
	if errors.As(err, &nfErr) {
		details := httpDetails(nfErr.Endpoint, nfErr.StatusCode, nfErr.RequestID)
		if nfErr.NodeID != "" {
			if details == nil {
				details = map[string]any{}
			}
			details["nodeId"] = aphotos.Redact(nfErr.NodeID)
		}
		return newError(ErrResourceNotFound, nfErr.Error(), details)
	}

	var conflictErr *aphotos.ConflictError
	if errors.As(err, &conflictErr) {
		details := httpDetails(conflictErr.Endpoint, conflictErr.StatusCode, conflictErr.RequestID)
		if conflictErr.ConflictingNodeID != "" {
			if details == nil {
				details = map[string]any{}
			}
			details["conflictingNodeId"] = aphotos.Redact(conflictErr.ConflictingNodeID)
		}
		return newError(ErrConflict, conflictErr.Error(), details)
	}

	var netErr *aphotos.TransientNetworkError
	if errors.As(err, &netErr) {
		return newError(ErrNetworkError, netErr.Error(), httpDetails(netErr.Endpoint, netErr.StatusCode, netErr.RequestID))
	}

	var protoErr *aphotos.RemoteProtocolError
	if errors.As(err, &protoErr) {
		return newError(ErrRemoteProtocolError, protoErr.Error(), httpDetails(protoErr.Endpoint, protoErr.StatusCode, protoErr.RequestID))
	}

	var partialErr *aphotos.PartialResultError
	if errors.As(err, &partialErr) {
		var details map[string]any
		if partialErr.Endpoint != "" {
			details = map[string]any{"endpoint": aphotos.Redact(partialErr.Endpoint)}
		}
		if len(partialErr.Warnings) > 0 {
			if details == nil {
				details = map[string]any{}
			}
			warnings := make([]string, len(partialErr.Warnings))
			for i, w := range partialErr.Warnings {
				warnings[i] = aphotos.Redact(w)
			}
			details["warnings"] = warnings
		}
		return newError(ErrPartialSuccess, partialErr.Error(), details)
	}

	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return newError(ErrFilesystemError, err.Error(), nil)
	}

	return newError(ErrValidationFailed, err.Error(), nil)
}

func httpDetails(endpoint string, status int, reqID string) map[string]any {
	var d map[string]any
	if endpoint != "" {
		d = map[string]any{"endpoint": aphotos.Redact(endpoint)}
	}
	if status > 0 {
		if d == nil {
			d = map[string]any{}
		}
		d["statusCode"] = status
	}
	if reqID != "" {
		if d == nil {
			d = map[string]any{}
		}
		d["requestId"] = aphotos.Redact(reqID)
	}
	return d
}

func sanitizeDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	out := make(map[string]any, len(details))
	for k, v := range details {
		switch val := v.(type) {
		case string:
			out[k] = aphotos.Redact(val)
		case []string:
			clean := make([]string, len(val))
			for i, s := range val {
				clean[i] = aphotos.Redact(s)
			}
			out[k] = clean
		default:
			out[k] = v
		}
	}
	return out
}
