// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	"os"
	"testing"
)

// TestGoldenWem asserts BuildWem reproduces a committed golden .wem byte-for-byte
// from a committed .ogg. The golden was produced by the Python reference
// implementation and verified identical to this package's output across the
// full 112-file corpus.
func TestGoldenWem(t *testing.T) {
	ogg, err := os.ReadFile("testdata/sample.ogg")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/sample.wem")
	if err != nil {
		t.Fatal(err)
	}
	got, err := BuildWem(ogg, Options{IncludeHash: true})
	if err != nil {
		t.Fatalf("BuildWem: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("byte %d differs: got 0x%02x want 0x%02x", i, got[i], want[i])
		}
	}
}

// TestCodebookLibraryLoads sanity-checks the embedded aoTuV codebook library.
func TestCodebookLibraryLoads(t *testing.T) {
	lib := library()
	if len(lib.byKey) < 400 {
		t.Fatalf("codebook library too small: %d entries", len(lib.byKey))
	}
}
