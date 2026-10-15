package view

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const errorToastTimeout = 6 * time.Second

func (a *App) toast(message string) {
	a.toasts = append(a.toasts, ui.Toast{Title: message})
}

func (a *App) toastAction(message, label string, run func()) {
	a.toasts = append(a.toasts, ui.Toast{
		Title:    message,
		Action:   label,
		OnAction: run,
		Timeout:  errorToastTimeout,
	})
}

func (a *App) toastError(err error) {
	a.toasts = append(a.toasts, ui.Toast{
		Title:   errMessage(err),
		Type:    "error",
		Timeout: errorToastTimeout,
	})
}

func (a *App) flushToasts(c *ui.Context) {
	for _, t := range a.toasts {
		c.AddToast(t)
	}
	a.toasts = nil
}
