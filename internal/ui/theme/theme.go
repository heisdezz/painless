// Package theme configures Lobe UI tokens for Painless.
package theme

import (
	"github.com/egoist/mygo/ui"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// Apply selects the Lobe tokens for the window's current appearance.
func Apply(c *ui.Context) { lobetheme.Apply(c) }

// Tokens returns the applied Lobe tokens for the current frame.
func Tokens(c *ui.Context) *lobetheme.Tokens { return lobetheme.TokensOf(c) }

// UsageColor colors a storage gauge: neutral until 85% full, a warning
// until 95%, then an error.
func UsageColor(usedPercentage float64, tk *lobetheme.Tokens) ui.Color {
	switch {
	case usedPercentage >= 95:
		return tk.Error
	case usedPercentage >= 85:
		return tk.Warning
	default:
		return tk.Primary
	}
}
