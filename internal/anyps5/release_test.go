// SPDX-License-Identifier: GPL-2.0-only

package anyps5

import (
	"runtime"
	"testing"
)

func fixtureRelease() *Release {
	return &Release{
		Tag: "v1.2.3", Name: "1.2.3",
		Assets: []Asset{
			{Name: "relinker-v1.2.3", Size: 10, Digest: "sha256:abc"},
			{Name: "relinker-v1.2.3.exe", Size: 11, Digest: "sha256:def"},
			{Name: "prx-linux-v1.2.3.tar.gz", Size: 12},
			{Name: "prx-windows-v1.2.3.tar.gz", Size: 13},
			{Name: "prx-linux-v1.2.3.zip", Size: 14},
			{Name: "USAGE.md", Size: 15},
		},
	}
}

func TestSelectBundlePicksHostRelinkerAndTargetLibraries(t *testing.T) {
	bundle, err := SelectBundle(fixtureRelease(), "windows")
	if err != nil {
		t.Fatalf("SelectBundle: %v", err)
	}
	if bundle.Libs.Name != "prx-windows-v1.2.3.tar.gz" {
		t.Fatalf("libs = %s", bundle.Libs.Name)
	}
	expected := "relinker-v1.2.3"
	if runtime.GOOS == "windows" {
		expected = "relinker-v1.2.3.exe"
	}
	if bundle.Relinker.Name != expected {
		t.Fatalf("relinker = %s, want %s", bundle.Relinker.Name, expected)
	}
	if len(bundle.Docs) != 1 || bundle.Docs[0].Name != "USAGE.md" {
		t.Fatalf("docs = %#v", bundle.Docs)
	}
}

func TestSelectBundleRejectsMissingTarget(t *testing.T) {
	release := fixtureRelease()
	release.Assets = release.Assets[:2]
	if _, err := SelectBundle(release, "linux"); err == nil {
		t.Fatal("expected an error for a release without system libraries")
	}
}

func TestDigestSHA(t *testing.T) {
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if got := digestSHA("sha256:" + digest); got != digest {
		t.Fatalf("digestSHA = %q", got)
	}
	if got := digestSHA("sha512:whatever"); got != "" {
		t.Fatalf("digestSHA accepted a non-SHA-256 digest: %q", got)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.5", "0.1.4", 1},
		{"v0.2", "0.10", -1},
		{"1.0", "1.0.0", 0},
	}
	for _, item := range cases {
		if got := compareVersions(item.a, item.b); got != item.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", item.a, item.b, got, item.want)
		}
	}
}
