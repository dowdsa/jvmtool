package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.3.3", "0.4.0", true},
		{"v0.3.3", "v0.4.0", true},
		{"0.4.0", "0.3.3", false},
		{"0.4.0", "0.4.0", false},
		{"0.4.0", "0.4.0-rc1", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.current, tc.latest); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestInstallerURL(t *testing.T) {
	rel := &Release{TagName: "v0.4.0", Assets: []Asset{
		{Name: "jm-desktop_windows_amd64.exe", DownloadURL: "https://example.com/jm.exe"},
	}}
	got, err := rel.InstallerURL("windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/jm.exe" {
		t.Fatalf("InstallerURL() = %q", got)
	}
	if _, err := rel.InstallerURL("windows", "arm64"); err == nil {
		t.Fatal("InstallerURL accepted an unavailable architecture")
	}
}

func TestCLIAssetName(t *testing.T) {
	if got := CLIAssetName("linux", "amd64"); got != "jm_linux_amd64" {
		t.Fatalf("CLIAssetName(linux/amd64) = %q", got)
	}
	if got := CLIAssetName("windows", "amd64"); got != "jm_windows_amd64.exe" {
		t.Fatalf("CLIAssetName(windows/amd64) = %q", got)
	}
}
