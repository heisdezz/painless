// Package mediacard renders the shared media tile used by the library grid and
// the dashboard's recent-media strip, so both stay visually identical.
package mediacard

import (
	"strings"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/overlay"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// Fixed tile metrics from the design system: 168x188 cells with a 148x110
// preview, 10px padding and an 8px internal gap.
const (
	CardWidth     float32 = 168
	CardHeight    float32 = 188
	PreviewWidth  float32 = 148
	PreviewHeight float32 = 110
)

// Reveal choreographs a staggered entrance for a batch of cards: each card
// waits Delay(index) from Start, then rises 8 DIP while fading in. Share one
// Reveal across the batch; nil means cards render without entrance motion.
type Reveal struct {
	Start    time.Time
	Per      time.Duration // added delay per card index
	Max      time.Duration // cap on the per-card slot delay
	Duration time.Duration // length of one card's rise and fade
}

// NewReveal starts a reveal choreography now.
func NewReveal(now time.Time, per, max, duration time.Duration) *Reveal {
	return &Reveal{Start: now, Per: per, Max: max, Duration: duration}
}

// Delay is the wait still owed to a card at index when it first builds: the
// card's slot minus the time already spent since Start.
func (r *Reveal) Delay(index int) time.Duration {
	slot := min(r.Per*time.Duration(index), r.Max)
	if elapsed := time.Since(r.Start); elapsed >= slot {
		return 0
	} else {
		return slot - elapsed
	}
}

// Active reports whether the choreography is still worth posing. After it
// returns false, cards render plainly and scrolled-in cards never re-animate.
func (r *Reveal) Active() bool {
	return time.Since(r.Start) < r.Max+r.Duration+150*time.Millisecond
}

// Props describes one media tile.
type Props struct {
	// Width fixes the tile width; zero lets the parent decide (grid cells).
	Width float32
	// Label is the accessible label used by testers and the a11y tree.
	Label string
	// Name is the file name shown under the preview.
	Name string
	// SizeLabel is the human-readable file size.
	SizeLabel string
	// DurationLabel is set for videos and rendered as a badge.
	DurationLabel string
	// FormatLabel, when set, renders a small Signal Blue format tag.
	FormatLabel string
	// Bitmap is the decoded preview; a skeleton shows while it is nil.
	Bitmap *mui.Bitmap
	// Selected draws the multi-select highlight.
	Selected bool
	// Items is the context menu; empty disables the menu.
	Items []overlay.MenuItem
	// OnClick receives the click modifiers (Cmd/Shift) for select semantics.
	OnClick func(mods mui.Modifiers)
	// OnContext receives the chosen context menu key.
	OnContext func(key string)
	// Reveal, when set, plays the card's staggered entrance; RevealIndex is
	// the card's slot in that choreography.
	Reveal      *Reveal
	RevealIndex int
}

// revealState is what a revealing card keeps between frames: whether it has
// entered, and its frozen slot delay, so later rebuilds (thumbnail loads) do
// not restart or re-delay a mid-flight entrance.
type revealState struct {
	entered bool
	delay   time.Duration
}

// Render draws a media tile. Callers set c.Key before calling when they need a
// stable identity.
func Render(c *mui.Context, p Props) {
	tk := lobetheme.TokensOf(c)
	card := mui.Column(c).Gap(8).Padding(10).Radius(tk.BorderRadius).AlignSelf(mui.Stretch)
	if p.Width > 0 {
		card.Width(p.Width)
	}
	if p.Label != "" {
		card.Label(p.Label)
	}
	if p.Selected {
		card.Background(tk.FillSecondary).Border(1, tk.Primary)
	} else {
		card.Border(1, tk.BorderSecondary)
	}
	card.Children(func() {
		if p.Bitmap != nil {
			mui.Image(c, p.Bitmap).Width(PreviewWidth).Height(PreviewHeight).Fit(mui.Cover).Radius(tk.BorderRadiusSM)
		} else {
			display.Skeleton(c, display.SkeletonProps{Width: PreviewWidth, Height: PreviewHeight})
		}
		mui.Text(c, p.Name).FontSize(12).SingleLine()
		mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
			if p.FormatLabel != "" {
				display.Tag(c, strings.ToUpper(p.FormatLabel), display.TagProps{
					Size: lobetheme.Small, Color: display.TagBlue,
				})
			}
			if p.SizeLabel != "" {
				mui.Text(c, p.SizeLabel).FontSize(11).TextColor(tk.TextTertiary)
			}
			if p.DurationLabel != "" {
				display.Tag(c, p.DurationLabel, display.TagProps{
					Size: lobetheme.Small, Color: display.TagGeekblue,
				})
			}
		})
	})
	if p.OnClick != nil {
		card.OnClick(func() { p.OnClick(card.ClickModifiers()) })
	}
	if len(p.Items) > 0 && p.OnContext != nil {
		if key := overlay.ContextMenu(c, card, overlay.DropdownMenuProps{Items: p.Items}); key != "" {
			p.OnContext(key)
		}
	}
	applyReveal(c, card, p)
}

// applyReveal plays the card's entrance: the first build shows it risen pose
// down and fully faded, asks for another frame, and the next build animates it
// to rest after its slot delay — the canonical two-frame enter from AnimatePose.
func applyReveal(c *mui.Context, card mui.Element, p Props) {
	ApplyReveal(c, card, p.Reveal, p.RevealIndex)
}

// ApplyReveal plays a staggered entrance on any element: the first build shows
// it at the hidden pose and asks for another frame, the next animates it to
// rest after its slot delay. Cards in a batch share one Reveal and pass their
// index; a nil reveal renders without motion. Used by the media tile itself
// and by the album gallery's own card chrome.
func ApplyReveal(c *mui.Context, e mui.Element, r *Reveal, index int) {
	if r == nil || !r.Active() {
		return
	}
	st := mui.Local(e, "painless.reveal", func() revealState { return revealState{} })
	pose := mui.Pose{}
	if !st.entered {
		st.entered = true
		st.delay = r.Delay(index)
		pose = mui.Pose{Y: 8, Fade: 1}
		c.Invalidate()
	}
	e.AnimatePose(pose, mui.Animation{Duration: r.Duration, Delay: st.delay})
}

// SkeletonTile renders one card-shaped placeholder with the shared tile's
// chrome and metrics: a sweeping preview block plus name and meta rows. Grids
// show these while their pages load, so the skeleton matches the real card
// exactly and the grid does not shift when content arrives.
func SkeletonTile(c *mui.Context) {
	tk := lobetheme.TokensOf(c)
	mui.Column(c).Gap(8).Padding(10).Radius(tk.BorderRadius).
		Width(CardWidth).AlignSelf(mui.Stretch).
		Border(1, tk.BorderSecondary).
		Children(func() {
			display.Skeleton(c, display.SkeletonProps{
				Width: PreviewWidth, Height: PreviewHeight,
				Animation: display.SkeletonSweep,
			})
			display.Skeleton(c, display.SkeletonProps{Width: 96, Height: 12})
			display.Skeleton(c, display.SkeletonProps{Width: 60, Height: 11})
		})
}
