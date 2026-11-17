# UI Feature 05: Discover Scanner View & Staging Consolidator

## 1. Overview
The Discover / Scanner view is where the user initiates filesystem indexing of an external drive or specific subfolder, monitors live scan progress, reviews discovered media in the virtual staging area, and commits physical file relocations via the **"Consolidate & Execute"** action.

---

## 2. Views & Sections

### 1. Scanner Control Header (`internal/ui/screens/discover/scan_controls.go`)
- Target folder picker (`ui.Button("Select Folder")` invoking native file dialog).
- Ignore list configuration (input tags for folders to skip).
- Action buttons: "Start Scan", "Pause", "Cancel".
- Live progress indicator:
  - Animated progress bar.
  - Counters: Total files scanned, media files discovered, elapsed time, current processing directory.

### 2. Virtual Staging Review Table (`internal/ui/screens/discover/staging_table.go`)
- Displays all newly discovered media files before they are physically moved.
- Columns:
  - Thumbnail preview.
  - File Name & Path.
  - Size & Type.
  - Target Assignment Dropdown: Assign to `[Album Name]`, `[Unknown]`, or `[Skip / Ignore]`.
- Batch assign buttons: "Assign all to Album...", "Send all unassigned to Unknown".

### 3. Consolidate & Execute Action Panel (`internal/ui/screens/discover/execute_panel.go`)
- Prominent bottom banner summarizing staged actions:
  - *"45 files will be moved to albums/Vacation 2026"*
  - *"12 files will be moved to unknown/"*
  - *"0 files will be overwritten"*
- Action Button: **"Consolidate & Execute"** (Primary accent styling).
- Confirmation Modal:
  - Details the exact disk modifications about to occur.
  - Requires user click on "Confirm & Write to Disk".
  - Shows execution progress bar with cancel/abort protection.

---

## 3. File Structure
- `internal/ui/screens/discover/discover_screen.go`: Main discover screen container.
- `internal/ui/screens/discover/scan_controls.go`: Scan trigger, folder selector, live status bar.
- `internal/ui/screens/discover/staging_table.go`: Staged media review and assignment table.
- `internal/ui/screens/discover/execute_panel.go`: Summary banner and execution confirmation modal.

---

## 4. Acceptance Criteria
- [ ] Folder picker opens native directory selection dialog.
- [ ] Scanning shows real-time progress without freezing the UI.
- [ ] Staging table allows assigning destination albums to discovered media.
- [ ] "Consolidate & Execute" executes file moves with live progress feedback and summary report.
