package ui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	usmsync "apps.z7.ai/usm/internal/sync"
)

func (a *app) makeDiscoveryView() fyne.CanvasObject {
	if a.syncService == nil || !a.syncService.IsRunning() {
		return container.NewCenter(widget.NewLabel("LAN Sync is not enabled. Enable it in Preferences."))
	}

	// "This Device" identity card
	thisDeviceCard := a.makeThisDeviceCard()

	trustedBox := container.NewVBox()
	discoveredBox := container.NewVBox()

	// Scanning indicator
	activity := NewActivity()
	activity.Start()
	scanningRow := container.NewHBox(activity, widget.NewLabel("Scanning local network..."))

	refresh := func(peers []usmsync.PeerInfo) {
		fyne.Do(func() {
			trustedBox.RemoveAll()
			discoveredBox.RemoveAll()
			for _, p := range peers {
				card := a.makePeerCard(p)
				if p.Trusted {
					trustedBox.Add(card)
				} else {
					discoveredBox.Add(card)
				}
			}
			// Empty state labels
			if len(trustedBox.Objects) == 0 {
				empty := widget.NewLabel("No trusted peers yet. Pair with a discovered device to get started.")
				empty.Wrapping = fyne.TextWrapWord
				empty.Importance = widget.LowImportance
				trustedBox.Add(empty)
			}
			if len(discoveredBox.Objects) == 0 {
				empty := widget.NewLabel("No other devices found on the local network.")
				empty.Importance = widget.LowImportance
				discoveredBox.Add(empty)
			}
			trustedBox.Refresh()
			discoveredBox.Refresh()
		})
	}

	// Load current peers
	refresh(a.syncService.Peers())

	// Subscribe to changes
	a.syncService.OnPeerChange(refresh)

	trustedSection := widget.NewCard("Trusted Peers", "", trustedBox)
	discoveredSection := widget.NewCard("Discovered Peers", "", discoveredBox)

	content := container.NewVScroll(
		container.NewVBox(
			thisDeviceCard,
			scanningRow,
			trustedSection,
			discoveredSection,
		),
	)

	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, content)
}

// makeThisDeviceCard builds the "This Device" identity header showing the local
// hostname, peer ID, and deterministic color dot so the user knows who they are.
func (a *app) makeThisDeviceCard() fyne.CanvasObject {
	hostID := a.syncService.HostID()

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "Unknown"
	}

	// Color dot from own peer ID
	dotColor := parseHexColour(usmsync.PeerColor(hostID))
	dot := canvas.NewCircle(dotColor)
	dot.StrokeWidth = 0
	dotContainer := container.New(layout.NewCenterLayout(), dot)
	dot.Resize(fyne.NewSquareSize(14))
	dotContainer.Resize(fyne.NewSize(18, 18))
	dotFixed := container.NewGridWrap(fyne.NewSquareSize(18), dotContainer)

	nameLabel := widget.NewLabel(hostname)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Truncated peer ID for display
	displayID := hostID
	if len(displayID) > 20 {
		displayID = displayID[:20] + "..."
	}
	idText := canvas.NewText(displayID, color.NRGBA{R: 160, G: 160, B: 160, A: 255})
	idText.TextSize = 11
	idText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	info := container.NewVBox(nameLabel, idText)

	row := container.NewBorder(nil, nil, dotFixed, nil, info)
	return widget.NewCard("This Device", "", row)
}

func (a *app) makePeerCard(peer usmsync.PeerInfo) fyne.CanvasObject {
	// Color dot
	dotColor := parseHexColour(usmsync.PeerColor(peer.ID))
	dot := canvas.NewCircle(dotColor)
	dot.StrokeWidth = 0
	dot.Resize(fyne.NewSquareSize(10))
	dotFixed := container.NewGridWrap(fyne.NewSquareSize(14),
		container.New(layout.NewCenterLayout(), dot))

	// Row 1: label (bold)
	nameLabel := widget.NewLabel(peer.Label)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	nameLabel.Truncation = fyne.TextTruncateEllipsis

	// Row 2: peer ID (small, bright gray, bold monospace)
	displayID := peer.ID
	if len(displayID) > 20 {
		displayID = displayID[:20] + "..."
	}
	// ATTN: canvas.NewText gives direct control over font size and color,
	// unlike widget.Label which only offers LowImportance (too dim).
	brightGray := color.NRGBA{R: 160, G: 160, B: 160, A: 255}
	idText := canvas.NewText(displayID, brightGray)
	idText.TextSize = 11
	idText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	// Row 3: trust metadata or status (small, bright gray)
	var metaStr string
	if peer.Trusted {
		if tp, ok := a.syncService.TrustStore().Get(peer.ID); ok {
			metaStr = formatTrustMeta(tp)
		} else {
			metaStr = "Trusted"
		}
	} else {
		metaStr = peer.Status.String()
	}
	metaText := canvas.NewText(metaStr, brightGray)
	metaText.TextSize = 11

	// Row 4: action buttons
	peerID := peer.ID
	peerLabel := peer.Label
	var actions fyne.CanvasObject
	if peer.Trusted {
		syncBtn := widget.NewButton("Sync", func() { a.startSync(peerID, peerLabel) })
		syncBtn.Importance = widget.HighImportance
		unpairBtn := widget.NewButton("Unpair", func() {
			dialog.NewConfirm("Unpair", fmt.Sprintf("Remove %s from trusted peers?", peerLabel), func(ok bool) {
				if !ok {
					return
				}
				if err := a.syncService.TrustStore().Remove(peerID); err != nil {
					dialog.ShowError(err, a.win)
					return
				}
				truncated := peerID
				if len(truncated) > 12 {
					truncated = truncated[:12] + "..."
				}
				a.syncService.SetTrusted(peerID, false, truncated)
				a.showDiscoveryView()
			}, a.win).Show()
		})
		unpairBtn.Importance = widget.DangerImportance
		actions = container.NewHBox(syncBtn, unpairBtn)
	} else {
		pairBtn := widget.NewButton("Pair", func() { a.startPairing(peerID, peerLabel) })
		pairBtn.Importance = widget.HighImportance
		actions = pairBtn
	}

	// ATTN: all 4 rows in a VBox, with the dot on the left via Border.
	// Previous layout put buttons in Border's right position, which stretched
	// them to full card height and forced the window wider. Buttons in the
	// VBox bottom row stay at natural size.
	info := container.NewVBox(nameLabel, idText, metaText, actions)
	return container.NewBorder(nil, nil, dotFixed, nil, info)
}

// formatTrustMeta builds a human-readable line from trust-store metadata.
func formatTrustMeta(tp usmsync.TrustedPeer) string {
	parts := "Paired " + relativeTime(tp.PairedAt)
	if !tp.LastSync.IsZero() {
		parts += "  ·  Last sync " + relativeTime(tp.LastSync)
	} else {
		parts += "  ·  Never synced"
	}
	return parts
}

// relativeTime returns a short human-readable description of how long ago t was.
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		if days < 30 {
			return fmt.Sprintf("%d days ago", days)
		}
		return t.Format("2 Jan 2006")
	}
}

// startPairing initiates a pairing handshake and shows the code dialog
func (a *app) startPairing(peerID, peerLabel string) {
	code, resultCh, err := a.syncService.InitiatePairing(context.Background(), peerID)
	if err != nil {
		// Peer unreachable — remove stale entry and refresh
		a.syncService.RemovePeer(peerID)
		a.showDiscoveryView()
		dialog.ShowError(err, a.win)
		return
	}

	codeLabel := widget.NewLabel(code)
	codeLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	codeLabel.Alignment = fyne.TextAlignCenter

	activity := NewActivity()
	activity.Start()

	content := container.NewVBox(
		widget.NewLabel("Enter this code on the remote device:"),
		codeLabel,
		container.NewHBox(activity, widget.NewLabel("Waiting for confirmation...")),
	)

	d := dialog.NewCustom("Pairing with "+peerLabel, "Cancel", content, a.win)
	d.Show()

	go func() {
		result := <-resultCh
		fyne.Do(func() {
			d.Hide()
			if result.Success {
				dialog.ShowInformation("Paired",
					fmt.Sprintf("Successfully paired with %s.", result.Label), a.win)
				a.showDiscoveryView()
			} else if result.Err != nil {
				// Remove unreachable peer from the list
				a.syncService.RemovePeer(peerID)
				a.showDiscoveryView()
				dialog.ShowError(result.Err, a.win)
			}
		})
	}()
}

// startSync initiates vault sync with a trusted peer
func (a *app) startSync(peerID, peerLabel string) {
	activity := NewActivity()
	activity.Start()

	content := container.NewVBox(
		container.NewHBox(activity, widget.NewLabel("Synchronising with "+peerLabel+"...")),
	)

	d := dialog.NewCustom("Sync", "Cancel", content, a.win)
	d.Show()

	go func() {
		result, err := a.syncService.SyncWithPeer(context.Background(), peerID)
		fyne.Do(func() {
			d.Hide()
			if err != nil {
				// ATTN: reload even on error — partial transfers may have changed disk state
				a.reloadStateFromDisk()
				dialog.ShowError(err, a.win)
				return
			}

			summary := fmt.Sprintf("Sync with %s complete.\n", peerLabel)
			if len(result.Transfers) > 0 {
				for _, t := range result.Transfers {
					summary += fmt.Sprintf("  %s: %s\n", t.VaultName, t.Direction)
				}
			} else {
				summary += "All vaults are already in sync."
			}
			if len(result.Errors) > 0 {
				summary += "\nErrors:\n"
				for _, e := range result.Errors {
					summary += "  " + e + "\n"
				}
			}

			// ATTN: show summary first, reload state when user dismisses it.
			// This ensures the user reads the result before the view changes.
			summaryLabel := widget.NewLabel(summary)
			summaryLabel.Wrapping = fyne.TextWrapWord
			sd := dialog.NewCustom("Sync Complete", "OK", summaryLabel, a.win)
			sd.SetOnClosed(func() {
				a.reloadStateFromDisk()
			})
			sd.Show()
		})
	}()
}

// handleIncomingPairRequest is called from the sync service when a remote peer
// initiates pairing. It shows a dialog and blocks until the user responds.
func (a *app) handleIncomingPairRequest(peerID, peerLabel string) (string, bool) {
	type response struct {
		code     string
		accepted bool
	}
	ch := make(chan response, 1)

	fyne.Do(func() {
		entry := widget.NewEntry()
		entry.PlaceHolder = "Enter pairing code"

		d := dialog.NewCustomConfirm(
			fmt.Sprintf("Pair with %s?", peerLabel),
			"Confirm", "Cancel",
			entry,
			func(ok bool) {
				if ok && entry.Text != "" {
					ch <- response{code: entry.Text, accepted: true}
				} else {
					ch <- response{accepted: false}
				}
			},
			a.win,
		)
		d.Show()
		a.win.RequestFocus()
	})

	resp := <-ch
	return resp.code, resp.accepted
}

// handleIncomingSync is called from the sync service when a remote peer starts
// pushing data to this device. It blocks until a lock dialog is displayed, then
// returns a doneFn that the service calls when the sync finishes. The doneFn
// dismisses the dialog, reloads all state from disk, and navigates to a safe view.
func (a *app) handleIncomingSync(peerID string) func(err error) {
	type readySignal struct{ d dialog.Dialog }
	ch := make(chan readySignal, 1)

	peerLabel := peerID
	if len(peerLabel) > 16 {
		peerLabel = peerLabel[:16] + "..."
	}
	if tp, ok := a.syncService.TrustStore().Get(peerID); ok {
		peerLabel = tp.Label
	}

	fyne.Do(func() {
		activity := NewActivity()
		activity.Start()
		content := container.NewVBox(
			container.NewHBox(activity, widget.NewLabel("Receiving data from "+peerLabel+"...")),
			widget.NewLabel("Please wait while the sync completes."),
		)
		// ATTN: non-cancellable sync dialog — the user must wait for completion
		// to ensure state consistency. An empty dismiss label hides the button.
		d := dialog.NewCustom("Sync in Progress", "", content, a.win)
		d.Show()
		ch <- readySignal{d: d}
	})

	// Block until the dialog is displayed
	sig := <-ch

	return func(syncErr error) {
		fyne.Do(func() {
			sig.d.Hide()
			if syncErr != nil {
				a.reloadStateFromDisk()
				dialog.ShowError(syncErr, a.win)
				return
			}
			// ATTN: show result first, reload state when user dismisses the dialog.
			// This ensures the user sees the outcome before the view changes.
			infoLabel := widget.NewLabel(
				fmt.Sprintf("Data received from %s.\nVaults have been updated.", peerLabel))
			infoLabel.Wrapping = fyne.TextWrapWord
			sd := dialog.NewCustom("Sync Complete", "OK", infoLabel, a.win)
			sd.SetOnClosed(func() {
				a.reloadStateFromDisk()
			})
			sd.Show()
		})
	}
}

func (a *app) showDiscoveryView() {
	a.setContentWithToolbar(a.makeDiscoveryView())
}
