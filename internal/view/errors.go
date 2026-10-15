package view

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

func errMessage(err error) string {
	var ge *gui.Error
	if errors.As(err, &ge) && ge.Message != "" {
		return ge.Message
	}
	if err == nil {
		return locales.ErrorGeneric()
	}
	return err.Error()
}

func errCode(err error) string {
	var ge *gui.Error
	if errors.As(err, &ge) {
		return ge.Code
	}
	return ""
}

func isAuthError(code string) bool {
	return code == gui.ErrAuthRequired || code == gui.ErrAuthFailed
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func friendlyError(m locales.Messages, code, raw string) (text, detail string) {
	if msg := classifyError(m, code, strings.ToLower(raw)); msg != "" {
		return msg, raw
	}
	return raw, ""
}

func classifyError(m locales.Messages, code, low string) string {
	switch code {
	case gui.ErrAuthRequired, gui.ErrAuthFailed, gui.ErrPermissionDenied:
		return m.ErrorSignIn()
	case gui.ErrRateLimited:
		return m.ErrorRateLimited()
	case gui.ErrNetworkError:
		return m.ErrorNetwork()
	}
	switch {
	case containsAny(low, "not found error", "(http 404)"):
		return m.ErrorMissing()
	case containsAny(low, "authentication error", "permission error", "(http 401)", "(http 403)"):
		return m.ErrorSignIn()
	case containsAny(low, "rate limit", "(http 429)"):
		return m.ErrorRateLimited()
	case containsAny(low, "transient network error", "no such host", "connection refused", "connection reset", "i/o timeout", "(http 502)", "(http 503)", "(http 504)"):
		return m.ErrorNetwork()
	}
	return ""
}

func errorText(c *ui.Context, code, raw string) ui.Element {
	text, detail := friendlyError(tr(c), code, raw)
	e := ui.Text(c, text)
	if detail != "" {
		e.Tooltip(detail)
	}
	return e
}
