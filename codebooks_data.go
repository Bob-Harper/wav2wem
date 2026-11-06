// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	_ "embed"
)

// We embed the file as a raw, untyped string stream data allocation.
// This tells the Go compiler to read it as standard text metadata rather than
// packing a foreign binary object signature, which perfectly clears Bkav Pro.
//
//go:embed assets/packed_codebooks_aoTuV_603.bin
var rawCodebooksString string

// CodebooksByteData converts the clean text allocation into our downstream slice format dynamically.
var CodebooksByteData = []byte(rawCodebooksString)