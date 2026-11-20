package shell

import (
	"testing"

	"painless/internal/domain"
	"painless/internal/state"
)

// TestDashboardOpenAlbumOpensAlbumDetail checks that clicking a recent album
// on the dashboard navigates to the albums screen with that album's detail
// opened.
func TestDashboardOpenAlbumOpensAlbumDetail(t *testing.T) {
	store := state.NewStore()
	shell := New(store)

	album := domain.AlbumSummary{Album: domain.Album{ID: 42, Name: "Trips"}, MediaCount: 3}
	dashActions := dashboardActions{store: store, shell: shell}
	dashActions.OpenAlbum(album)

	if got := store.State.Screen(); got != state.ScreenAlbums {
		t.Fatalf("screen after dashboard OpenAlbum = %q, want %q", got, state.ScreenAlbums)
	}
	if got := shell.Albums().SelectedAlbum(); got == nil || got.ID != album.ID || got.Name != album.Name {
		t.Fatalf("selected album = %+v, want %+v", got, album.Album)
	}

	// A dashboard without a bound shell still navigates; it shows the album gallery.
	noShell := dashboardActions{store: store}
	noShell.OpenAlbum(domain.AlbumSummary{Album: domain.Album{ID: 7, Name: "Solo"}})
	if got := store.State.Screen(); got != state.ScreenAlbums {
		t.Fatalf("screen after shell-less OpenAlbum = %q, want %q", got, state.ScreenAlbums)
	}
}
