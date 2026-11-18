---
name: Painless
description: Native black-box-theater media organizer for external drives
colors:
  stage-black: "#000000"
  panel-black: "#0d0d0d"
  raised-panel: "#1a1a1a"
  spotlight: "#2d2d2d"
  hairline: "#202020"
  hairline-soft: "#1a1a1a"
  ink: "#e6e6e6"
  ink-secondary: "#b4b4b4"
  ink-tertiary: "#8f8f8f"
  ink-faint: "#6b6b6b"
  stage-white: "#eeeeee"
  paper-white: "#ffffff"
  paper-layout: "#f8f8f8"
  paper-border: "#e3e3e3"
  bench-black: "#222222"
  ink-on-paper: "#1f1f1f"
  ink-on-paper-secondary: "#666666"
  ink-on-paper-tertiary: "#8a8a8a"
  signal-blue: "#60b1ff"
  signal-blue-deep: "#0559ff"
  info-blue: "#4cb3ff"
  status-green: "#80c933"
  status-amber: "#ffbe45"
  status-red: "#e95350"
typography:
  micro:
    fontFamily: "system sans (MyGo default)"
    fontSize: "9px"
    fontWeight: 800
    letterSpacing: "1px"
  label:
    fontFamily: "system sans (MyGo default)"
    fontSize: "11px"
    fontWeight: 600
    letterSpacing: "0.5px"
  caption:
    fontFamily: "system sans (MyGo default)"
    fontSize: "12px"
  body:
    fontFamily: "system sans (MyGo default)"
    fontSize: "14px"
    fontWeight: 400
  title:
    fontFamily: "system sans (MyGo default)"
    fontSize: "18px"
    fontWeight: 800
  heading:
    fontFamily: "system sans (MyGo default)"
    fontSize: "22px"
    fontWeight: 600
rounded:
  xs: "4px"
  sm: "6px"
  md: "8px"
  lg: "12px"
  tile: "14px"
  banner: "16px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
  screen: "28px"
components:
  media-card:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "10px"
  media-card-selected:
    backgroundColor: "{colors.spotlight}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "10px"
  format-tag:
    backgroundColor: "{colors.signal-blue}"
    textColor: "{colors.stage-black}"
    rounded: "{rounded.xs}"
  indexing-badge:
    backgroundColor: "{colors.panel-black}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "6px 12px"
  inspector-rail:
    backgroundColor: "{colors.stage-black}"
    textColor: "{colors.ink-secondary}"
    width: "300px"
  viewer-top-bar:
    backgroundColor: "{colors.stage-black}"
    textColor: "{colors.ink}"
    height: "52px"
    padding: "0px 12px"
---

# Design System: Painless

## Overview

**Creative North Star: "The Black Box Theater"**

Painless is a dark room where the media performs. The interface is the theater's architecture — walls, wings, and an inspector's rail — built so that nothing competes with the photograph or the frame on stage. Chrome recedes: surfaces are black or near-black panels, separated by 1px hairlines rather than shadows, and the brightest surface on any screen is the media itself. When the OS asks for light mode, the theater re-lights as a bright studio — the same flat, hairline construction, inverted to paper white with near-black chrome.

The system's one structural indulgence is the inspector rail: a fixed 300px column of quiet, right-aligned facts (filename, SHA-256, EXIF, GPS) that sits beside the stage like production notes taped to the wing. Buttons and controls are slim and monochrome — instrument-panel restraint. Controls appear only where work happens (a slim transport row at the foot of the video stage), never as permanent decoration. Color is rationed: near-white is the "accent" in dark mode, and the blue family appears only as informational tags and status; success green, warning amber, and error red are strictly semantic.

Density is high but calm: micro-labels at 9–10px uppercase with wide letter-spacing, UI text at 11–14px, and generous structural padding (24–28px) around content zones. Depth is never faked with glow — a surface is either a different tonal step or it is divided by a hairline.

**Key Characteristics:**
- Flat black surfaces, 1px hairline borders, zero shadows
- Monochrome primary accent (near-white on dark, near-black on light); blue reserved for tags/info
- Fixed inspector rail (300px) for metadata; media stage takes all remaining space
- Small, quiet UI type (9–14px) with uppercase micro-labels at 700–800 weight
- Tonal layering for elevation (BgBase → BgContainer → BgElevated → fills)

## Colors

A near-monochrome grayscale stage where color appears only as signal — status semantics, informational tags, and the media's own imagery.

### Primary
- **Stage White** (#eeeeee): the dark-mode primary — filled buttons, active controls, the drive icon. Inverted in light mode as **Bench Black** (#222222). The primary is chrome, not decoration: it marks what is interactive, nothing else.

### Secondary
- **Signal Blue** (#60b1ff dark / #0559ff light): informational tags (format badges like JPEG/MP4), links, and focus hue. Never used decoratively or as a large fill.

### Tertiary
- **Status Green** (#80c933): success and healthy-state text/icons only.
- **Status Amber** (#ffbe45): warnings; storage gauge turns amber at ≥85% full.
- **Status Red** (#e95350): errors and destructive actions (delete confirmations).
- **Info Blue** (#4cb3ff): informational banners and the info affordance in the viewer.

### Neutral (dark mode)
- **Stage Black** (#000000): window background and the media stage — the deepest plane.
- **Panel Black** (#0d0d0d): contained surfaces (cards' parent containers, toolbars).
- **Raised Panel** (#1a1a1a): the brightest structural surface — list headers, action bars, album tiles, modal chrome.
- **Spotlight** (#2d2d2d): hover and selected fills over panels (also expressed as white-alpha fills, 10–16%).
- **Hairline** (#202020): all 1px borders; **Hairline Soft** (#1a1a1a) for borders on already-raised surfaces.
- **Ink** (#e6e6e6): primary text. **Ink Secondary** (#b4b4b4): supporting text. **Ink Tertiary** (#8f8f8f): metadata, counters, placeholders' neighbors. **Ink Faint** (#6b6b6b): disabled and placeholder text.

### Neutral (light mode)
- **Paper White** (#ffffff): window background. **Paper Layout** (#f8f8f8): layout wells behind content.
- **Paper Border** (#e3e3e3): hairlines (soft variant #eeeeee).
- **Ink on Paper** (#1f1f1f): primary text; secondary #666666; tertiary #8a8a8a; quaternary #b0b0b0.

### Named Rules
**The Receding Chrome Rule.** No structural surface may be brighter than Raised Panel (#1a1a1a) in dark mode. If a UI element draws more attention than the media beside it, it is wrong.

**The Monochrome Voice Rule.** The primary accent is grayscale. Chromatic color (blue, green, amber, red) speaks only for meaning — status, format, information — never for style. If a colored element carries no semantic job, make it ink.

## Typography

**Display Font:** system sans (MyGo default)
**Body Font:** system sans (MyGo default)
**Label/Mono Font:** none — no distinct mono is in use yet; hashes and paths render as small ink-tertiary text

**Character:** One quiet voice at many volumes. The interface whispers in 11–14px; the largest type on screen belongs to content, not chrome. Weight (600–800), letter-spacing, and case — not size — create hierarchy.

### Hierarchy
- **Heading** (600, 22px): screen titles (e.g. placeholder/empty-state titles).
- **Title** (800, 18px): drive name, app wordmark in the drive bar; 700–800 weight plus 10px uppercase micro-labels (letter-spacing 1px) for eyebrow labels like "INDEXING ACTIVE".
- **Body** (400, 14px): default text, control labels, storage figures.
- **Caption** (400, 12–13px): card titles (12px), filenames in the viewer top bar (13px w600), counters and secondary metadata (12px).
- **Label** (600, 11px, letter-spacing 0.5px, sentence case): inspector-rail section headers ("MEDIA DETAILS & EXIF" reads uppercase via content); 11px also serves card metadata in Ink Tertiary.

### Named Rules
**The Small Chrome Rule.** Interface text lives between 9px and 14px. Larger sizes are reserved for content titles and the drive identity — never for buttons or labels.

## Layout

Full-bleed rows and columns anchored to window edges; the media stage always grows (`Grow(1), Basis(0)`) while rails and bars stay fixed. Standard construction: a fixed-height top bar (52px in the viewer), a `Grow` stage, and optional fixed rails (300px inspector) or foot rows (slim transport controls). Screens breathe with 24–28px structural padding (dashboard: 28px sides, 40px bottom; modal interiors: 8px vertical rhythm with 12–14px gaps).

The spacing rhythm is a 4px base: 4 / 8 / 12 / 16 / 24, with 28px for screen margins and occasional 40px for bottom breathing room. Component-internal gaps run 4–8px; sibling-component gaps run 12–16px; zone padding 24–28px.

There are no CSS breakpoints — this is a resizable desktop window. Verified fit checkpoints: the catalog filter bar must fit at 922px window width (search field grows/basis 220, min 140, max 360; count and sort shrink to 0), and the viewer must remain usable at 800×600 (rail squeezes the stage, never wraps).

## Elevation & Depth

**No shadows.** The system is flat by construction: depth is expressed exclusively through tonal layering (Stage Black → Panel Black → Raised Panel, plus white-alpha fills of 2–16% for hover/selected states) and 1px hairline borders (`Border`/`BorderSecondary`). Overlays (modals, drawers) use a 44% black mask rather than shadow to separate from the page. If a design feels like it needs a shadow, it needs a tonal step or a border instead.

### Named Rules
**The Flat Stage Rule.** Surfaces never cast shadows. A new layer earns presence by one tonal step up (or a hairline), never by elevation.

## Shapes

Soft but tight corners carry the form language. The working radius is 8px (`BorderRadius`) for cards, badges, and containers; 6px (`SM`) for inner elements like thumbnails and small tiles; 12px (`LG`) for larger containers; 4px (`XS`) for the smallest chips. Two structural exceptions recur: 14px for square icon tiles (the 48×48 drive icon) and 16px for banner surfaces (the dashboard drive header). Borders are always 1px hairlines in `Border` or `BorderSecondary`; there are no thick outlines, no double borders, and no clipped/organic shapes.

## Components

### Media Card (catalog grid)
- **Shape:** 8px radius, 10px padding, 8px internal gap
- **Thumbnail:** 148×110, 6px radius, cover fit
- **Text:** filename 12px single-line; size/type 11px Ink Tertiary in a 6px-gap row
- **Selected:** Spotlight-toned fill + 1px Primary (stage-white) border
- **Grid:** 168×188 cells, 8px gap; single click opens the viewer, Cmd/Shift-click selects

### Inspector Rail (signature component)
- **Shape:** fixed 300px column on the stage-black background, sections divided by hairlines, no card chrome
- **Header:** format tag (Signal Blue, 4px radius, uppercase MIME subtype) + size in a row
- **Sections:** 11px/600/0.5px-tracking headers in Ink Tertiary ("File Attributes", "Dimensions & Format", "EXIF Camera Details", "GPS Location")
- **Rows:** key left / value right-aligned (`TextAlign: End`); long values (paths, hashes) stack as wrapping blocks; copy affordances on SHA-256 and GPS coordinates
- **Behavior:** defaults open; toggled from the viewer top bar's info icon

### Viewer Top Bar
- **Shape:** borderless 52px row, 12px horizontal padding, 8px gaps — no background, no card
- **Content:** back arrow, filename (13px w600) with "n of m" counter (12px Ink Tertiary) adjacent, grow spacer, prev/next/info/close icons
- **Never** boxed, bordered, or given a distinct background

### Video Transport Controls
- **Shape:** slim unboxed row at the stage foot — play/pause icon, plain "00:00 / 02:05" time text, grow slider, mute icon
- **Never** a boxed card or floating pill; the row sits directly on the stage

### Buttons / Icon Buttons
- **Shape:** 8px radius; primary filled uses Primary (Stage White on dark / Bench Black on light) with contrast text; quiet actions are bare icons at ~16–20px in Ink Tertiary, brightening to Ink on hover
- **Badges:** "INDEXING ACTIVE" style — 6×12px padding, 8px radius, PrimaryBg fill, 1px PrimaryBorder, 9px w800 uppercase, 1px letter-spacing

### Inputs / Fields
- **Search:** grows within its bar (basis 220px, min 140px, max 360px); compact control heights from the token scale (36px standard, 27px small)
- **States:** disabled fills are 6% white-alpha (dark) with faint text; focus follows Lobe component defaults — no custom glow

### Navigation (sidebar)
- **Shape:** 12–14px padding columns, 6px gaps, 11–12px labels in Ink Secondary/Tertiary, single-line with truncation
- **Storage gauge:** neutral Primary color, Amber ≥85%, Red ≥95%

### Tags
- **Style:** Signal Blue family (TagBlue), 4px radius, tiny uppercase text (JPEG, MP4); used for format identity in the rail and lists — never for decoration

## Do's and Don'ts

### Do:
- **Do** consume Lobe tokens (`tk.*`) for every color, radius, and font size — no hardcoded hex in UI code.
- **Do** build chrome as borderless bars and rows with 1px hairline dividers; let the media own the brightest pixels.
- **Do** use tonal steps (BgBase → BgContainer → BgElevated) or hairlines for depth; state changes use alpha fills (10–16%).
- **Do** verify renders in dark mode (the flagship) at narrow widths — 800×600 viewer and 922px catalog are the regression checkpoints.
- **Do** keep UI text at 9–14px and create hierarchy with weight, case, and letter-spacing.

### Don't:
- **Don't** design like a website — no boxed cards around player controls, no per-row icons in the metadata rail, no web-page chrome (boxed headers, pill toolbars). This is a binding product commitment.
- **Don't** add box-shadows or glow; the system is flat by construction.
- **Don't** use chromatic color decoratively — blue is for tags/info, green/amber/red are semantic only.
- **Don't** let structural chrome exceed Raised Panel (#1a1a1a) in dark mode.
- **Don't** wrap the inspector rail or stack it below the stage; it is a fixed 300px column that squeezes the stage.
