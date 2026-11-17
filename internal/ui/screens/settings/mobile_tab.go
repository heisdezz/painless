package settings

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/code"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

// syncMobile pushes draft edits into the mobile controller. Lobe inputs
// mutate their bound fields in place, so side effects run by comparing
// against the last applied values — the same pattern the catalog uses to
// debounce its search box.
func (s *Screen) syncMobile() {
	if s.mobile == nil {
		return
	}
	if s.serverOn != s.lastOn {
		s.lastOn = s.serverOn
		s.mobile.SetEnabled(s.serverOn)
		if s.serverOn {
			s.raise(domain.LogLevelInfo, "Mobile sync started",
				"Pair from the companion app with the QR code or URL.")
		} else {
			s.raise(domain.LogLevelInfo, "Mobile sync stopped", "The LAN server is no longer listening.")
		}
	}
	if port := int(s.portValue); port != s.lastPort {
		s.lastPort = port
		s.mobile.SetPort(port)
	}
	// Follow token changes made outside this tab; edits typed into the
	// masked field are display-only and never written back.
	if token := s.mobile.Token(); token != s.lastToken {
		s.lastToken = token
		s.tokenDraft = token
	}
}

// mobileTab renders the server toggle, port, LAN pairing card and write
// token controls.
func (s *Screen) mobileTab(c *mui.Context) {
	if s.mobile == nil {
		emptyController(c, lucide.Smartphone(), "Mobile sync unavailable",
			"No mobile sync controller is wired into this build.")
		return
	}
	tk := lobetheme.TokensOf(c)

	mui.Column(c).Fill().Gap(12).Children(func() {
		card(c, "Server", func() {
			mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
				mui.Column(c).Grow(1).Gap(2).Children(func() {
					mui.Text(c, "Enable mobile sync").FontSize(13)
					mui.Text(c, "Serves the library over LAN for the companion app.").
						FontSize(12).TextColor(tk.TextSecondary)
				})
				choice.Switch(c, &s.serverOn, choice.SwitchProps{})
			})

			mui.Row(c).Shrink(0).Gap(12).AlignItems(mui.Center).Children(func() {
				mui.Column(c).Grow(1).Gap(2).Children(func() {
					mui.Text(c, "Listen port").FontSize(13)
					mui.Text(c, "Applies immediately; connected devices must reconnect.").
						FontSize(12).TextColor(tk.TextSecondary)
				})
				field.InputNumber(c, &s.portValue, field.InputNumberProps{
					Min:  1,
					Max:  65535,
					Step: 1,
				})
			})
		})

		s.pairingCard(c, tk)
		s.tokenCard(c, tk)

		if feedback.Modal(c, &s.regenOpen, feedback.ModalProps{
			Title:      "Regenerate write token?",
			OkText:     "Regenerate token",
			CancelText: "Cancel",
		}, func() {
			mui.Text(c, "The current write token stops working immediately. "+
				"Paired devices must pair again with the new token.").
				FontSize(13).TextColor(tk.TextSecondary)
		}) {
			s.regenOpen = false
			token := s.mobile.RegenerateToken()
			s.tokenDraft, s.lastToken = token, token
			s.raise(domain.LogLevelInfo, "Token regenerated",
				"Devices paired with the old token must reconnect.")
		}
	})
}

// pairingCard shows the QR code and copyable LAN URL once the server has
// a routable address.
func (s *Screen) pairingCard(c *mui.Context, tk *lobetheme.Tokens) {
	card(c, "LAN pairing", func() {
		url := s.mobile.URL()
		if url == "" {
			display.Empty(c, display.EmptyProps{
				Icon:        lucide.Wifi(),
				Title:       "No pairing address",
				Description: "Enable the server above to expose the LAN pairing URL and QR code.",
			})
			return
		}
		mui.Row(c).Shrink(0).Gap(16).AlignItems(mui.Center).Children(func() {
			code.QRCode(c, url, code.QRCodeProps{Size: 140})
			mui.Column(c).Grow(1).Gap(8).Children(func() {
				mui.Text(c, "Scan with the mobile app, or copy the URL:").
					FontSize(12).TextColor(tk.TextSecondary)
				mui.Row(c).Shrink(0).Gap(8).AlignItems(mui.Center).Children(func() {
					mui.Text(c, url).FontSize(13).Grow(1).SingleLine()
					action.CopyButton(c, url, action.CopyButtonProps{Size: lobetheme.Small})
				})
			})
		})
	})
}

// tokenCard shows the masked write token with copy and regenerate
// actions. The token is generated server-side; the field is read-only.
func (s *Screen) tokenCard(c *mui.Context, tk *lobetheme.Tokens) {
	card(c, "Write token", func() {
		mui.Text(c, "Required for uploads and edits from the companion app; reads stay open.").
			FontSize(12).TextColor(tk.TextSecondary)
		mui.Row(c).Shrink(0).Gap(8).AlignItems(mui.Center).Children(func() {
			field.InputPassword(c, &s.tokenDraft, field.InputProps{Size: lobetheme.Small}).
				Grow(1).ReadOnly(true)
			action.CopyButton(c, s.mobile.Token(), action.CopyButtonProps{Size: lobetheme.Small})
			action.Button(c, "Regenerate", action.ButtonProps{Size: lobetheme.Small}).OnClick(func() {
				s.regenOpen = true
			})
		})
	})
}
