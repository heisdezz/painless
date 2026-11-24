package domain

import "testing"

func TestIsSystemBootPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/boot", want: true},
		{path: "/boot/efi", want: true},
		{path: "/boot/EFI", want: true},
		{path: "/boot/grub2", want: true},
		{path: "/efi", want: true},
		{path: "/efi/boot", want: true},
		{path: "/esp", want: true},
		{path: "/sys/firmware/efi/efivars", want: true},
		{path: "/proc", want: true},
		{path: "/dev", want: true},
		{path: "/run/initramfs/live", want: true},
		{path: "/snap/core22/123", want: true},
		{path: "/Volumes/EFI", want: true},
		{path: "/System/Volumes/Preboot", want: true},
		{path: "/System/Volumes/Recovery", want: true},

		{path: "", want: false},
		{path: "/", want: false},
		{path: "/run/media/destiny/Ventoy", want: false},
		{path: "/media/user/USB", want: false},
		{path: "/bootleg-movies", want: false},
		{path: "/efilms", want: false},
		{path: "/home/user/Photos", want: false},
	}

	for _, tc := range tests {
		if got := IsSystemBootPath(tc.path); got != tc.want {
			t.Errorf("IsSystemBootPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
		if got := (DriveInfo{Path: tc.path}).IsSystemBoot(); got != tc.want {
			t.Errorf("DriveInfo{Path: %q}.IsSystemBoot() = %v, want %v", tc.path, got, tc.want)
		}
	}
}
