# UI Feature 05: Discover Scanner View & Staging Consolidator

## 1. Overview

The Discover / Scanner view is where users initiate filesystem scanning of an external drive or subfolder, watch live scanning progress (`feedback.Progress`), review discovered items in a virtual staging table, and trigger the physical file moves via the **"Consolidate & Execute"** action with modal confirmation (`overlay.Modal`).

---

## 2. Views & Sections

### 1. Scanner Control Bar (`internal/ui/screens/discover/scan_controls.go`)

- Target Folder Selector:
  - `field.Input` displaying chosen path.
  - `action.Button` ("Browse Folder...") invoking native file dialog.
- Ignore List:
  - `display.Tag` chips displaying ignored folder patterns (`node_modules`, `.git`, etc.) with remove buttons.
  - `field.Input` to add new ignore patterns.
- Scan Actions:
  - `action.Button`: "Start Scan" (Primary, `lucide.Play`)
  - `action.Button`: "Pause" (`lucide.Pause`)
  - `action.Button`: "Cancel" (Danger, `lucide.Square`)
- Live Progress Indicator:
  - `github.com/lobehub/lobe-mygo/pkg/feedback.Progress`: Animated line showing percentage progress with moving shine.
  - Stat counters (`display.Tag`): Scanned files count, media discovered count, elapsed time.
  - Current processing directory displayed with ellipsis truncation.

### 2. Virtual Staging Review Table (`internal/ui/screens/discover/staging_table.go`)

- Shows discovered media files staged for disk reorganization.
- Columns:
  - Preview thumbnail (`ui.Image`).
  - File Name & Original Path.
  - File Size & Type (`display.Tag`).
  - Target Assignment Dropdown (`field.Select`):
    - Assign to `[Album Name]`
    - Assign to `[Unknown / Unorganized]`
    - Skip / Do Not Move
- Batch action toolbar:
  - `action.Button`: "Assign All to Album..."
  - `action.Button`: "Send All to Unknown"

### 3. Consolidate & Execute Action Panel (`internal/ui/screens/discover/execute_panel.go`)

- Floating bottom banner summarizing staged actions:
  - `feedback.Alert`: Summary of files to move, directories to create, and unknown fallback counts.
  - `action.Button`: **"Consolidate & Execute"** (Primary accent, `lucide.CheckCircle2`).
- Confirmation Modal (`overlay.Modal`):
  - Frosted backdrop dialog displaying detailed file move breakdown.
  - Warning that physical file moves will be written to disk.
  - Requires clicking "Confirm & Move Files".
  - Shows live execution progress bar (`feedback.Progress`) during transfer.

---

## 3. File Structure

- `internal/ui/screens/discover/discover_screen.go`: Main container view.
- `internal/ui/screens/discover/scan_controls.go`: Target path picker and `feedback.Progress`.
- `internal/ui/screens/discover/staging_table.go`: Virtual staging review table with `field.Select`.
- `internal/ui/screens/discover/execute_panel.go`: Summary alert and `overlay.Modal` confirmation.

---

## 4. Acceptance Criteria

- [ ] Folder picker opens native dialog and updates target path.
- [ ] Scanning displays real-time `feedback.Progress` bar and file counters.
- [ ] Staging table allows assigning destination albums via `field.Select`.
- [ ] "Consolidate & Execute" presents confirmation `overlay.Modal` and commits moves to disk.
