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
		Data:    err,
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

func toastViewport(c *ui.Context) {
	t := c.Theme()
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Padding(24).AlignItems(ui.Center).Gap(8)
		for _, toast := range toasts {
			parts := ui.ToastBase(c, toast)
			parts.Root.Row().AlignItems(ui.Center).Gap(16).Padding(10, 16).Radius(radiusField).MaxWidth(600).Background(t.Surface).Border(1, t.Border).Children(func() {
				if err, ok := toast.Data.(error); ok {
					errorText(c, errCode(err), toast.Title).Grow(1).MinWidth(0)
				} else {
					ui.Text(c, toast.Title).Grow(1).MinWidth(0)
				}
				if toast.Action != "" {
					parts.ActionButton().Children(func() { ui.Text(c, toast.Action) })
				}
			})
		}
	})
}
