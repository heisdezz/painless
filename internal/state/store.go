package state

import (
	"painless/internal/domain"
)

// Store couples the central AppState with the event Bus. Background
// workers use its mutators, which update state and publish the matching
// event; the UI bridge subscribes to the bus and forwards events onto the
// main thread through mygo Window.Update.
type Store struct {
	State *AppState
	Bus   *Bus
}

// NewStore creates an application store with a fresh state and bus.
func NewStore() *Store {
	return &Store{State: New(), Bus: NewBus()}
}

// Selections exposes the selection store for direct read/mutation.
func (st *Store) Selections() *Selection { return &st.State.selection }

// ThumbnailsEnabled reports whether media thumbnails are displayed.
func (st *Store) ThumbnailsEnabled() bool { return st.State.ThumbnailsEnabled() }

// SetThumbnailsEnabled toggles the display of media thumbnails.
func (st *Store) SetThumbnailsEnabled(enabled bool) { st.State.SetThumbnailsEnabled(enabled) }

// AutoplayEnabled reports whether videos start playing when opened.
func (st *Store) AutoplayEnabled() bool { return st.State.AutoplayEnabled() }

// SetAutoplayEnabled toggles autoplay for videos opened in the viewer.
func (st *Store) SetAutoplayEnabled(enabled bool) { st.State.SetAutoplayEnabled(enabled) }

// ApplyDrives replaces the drive list and publishes EventDrivesChanged,
// plus EventActiveDriveChange when the active drive changed (including
// fallback selection after the active drive was removed).
func (st *Store) ApplyDrives(drives []domain.DriveInfo) {
	prevID, _ := st.State.SetDrives(drives)
	newID := st.State.ActiveDriveID()
	st.Bus.Publish(Event{Kind: EventDrivesChanged, Detail: newID})
	if prevID != newID {
		st.Bus.Publish(Event{Kind: EventActiveDriveChange, DriveID: newID})
	}
}

// SetActiveDrive selects a drive and publishes EventActiveDriveChange when
// it changed.
func (st *Store) SetActiveDrive(id string) {
	if st.State.SetActiveDrive(id) {
		st.Bus.Publish(Event{Kind: EventActiveDriveChange, DriveID: id})
	}
}

// ClearActiveDrive detaches the active drive and publishes
// EventActiveDriveChange when one was active.
func (st *Store) ClearActiveDrive() {
	if st.State.ClearActiveDrive() {
		st.Bus.Publish(Event{Kind: EventActiveDriveChange})
	}
}

// Navigate switches screens and publishes EventScreenChanged when it
// changed.
func (st *Store) Navigate(scr Screen) {
	if st.State.SetScreen(scr) {
		st.Bus.Publish(Event{Kind: EventScreenChanged, Detail: string(scr)})
	}
}

// ClearSelection empties the selection and publishes
// EventSelectionChanged.
func (st *Store) ClearSelection() {
	st.Selections().Clear()
	st.Bus.Publish(Event{Kind: EventSelectionChanged})
}

// ReplaceSelection sets the selection to ids and publishes
// EventSelectionChanged.
func (st *Store) ReplaceSelection(ids []int64) {
	st.Selections().Set(ids)
	st.Bus.Publish(Event{Kind: EventSelectionChanged})
}

// ToggleSelect toggles one id and publishes EventSelectionChanged.
func (st *Store) ToggleSelect(id int64) {
	st.Selections().Toggle(id)
	st.Bus.Publish(Event{Kind: EventSelectionChanged})
}

// RangeSelect selects the inclusive id range and publishes
// EventSelectionChanged.
func (st *Store) RangeSelect(start, end int64) {
	st.Selections().AddRange(start, end)
	st.Bus.Publish(Event{Kind: EventSelectionChanged})
}

// StartJobTracked registers a job and publishes EventJobStarted.
func (st *Store) StartJobTracked(id, kind, label string) {
	st.State.StartJob(id, kind, label)
	job, _ := st.State.Job(id)
	st.Bus.Publish(Event{Kind: EventJobStarted, Job: &job})
}

// ProgressJob updates a job and publishes EventJobProgress.
func (st *Store) ProgressJob(id string, progress float64, indeterminate bool, detail string) {
	st.State.UpdateJob(id, progress, indeterminate, detail)
	if job, ok := st.State.Job(id); ok {
		st.Bus.Publish(Event{Kind: EventJobProgress, Job: &job})
	}
}

// CompleteJob finishes a job and publishes EventJobFinished.
func (st *Store) CompleteJob(id string, status JobStatus, detail string) {
	st.State.FinishJob(id, status, detail)
	if job, ok := st.State.Job(id); ok {
		st.Bus.Publish(Event{Kind: EventJobFinished, Job: &job})
	}
}

// Notify publishes a toast event without state changes.
func (st *Store) Notify(level domain.LogLevel, title, body string) {
	st.Bus.Publish(Event{Kind: EventToast, Toast: &Toast{Level: level, Title: title, Body: body}})
}

// MediaChanged publishes EventMediaChanged after library mutations.
func (st *Store) MediaChanged(detail string) {
	st.Bus.Publish(Event{Kind: EventMediaChanged, Detail: detail})
}

// StagingChanged publishes EventStagingChanged after staging mutations.
func (st *Store) StagingChanged(detail string) {
	st.Bus.Publish(Event{Kind: EventStagingChanged, Detail: detail})
}
