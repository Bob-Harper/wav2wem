// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	_ "embed"
	"sync"
)

// Embedded assets. See NOTICE for licensing:
//   - packed_codebooks_aoTuV_603.bin: from ww2ogg (BSD, Xiph/Adam Gashlin),
//     codebooks derived from aoTuV (BSD).
//   - oggenc2.exe / libFLAC.dll: RareWares build of vorbis-tools (GPLv2) +
//     libvorbis/libogg/aoTuV/FLAC (BSD). The GPLv2 binary is redistributed
//     here under GPLv2; see NOTICE for the source offer.

//go:embed assets/packed_codebooks_aoTuV_603.bin
var codebooksData []byte

//go:embed assets/oggenc2.exe
var oggencExe []byte

//go:embed assets/libFLAC.dll
var libflacDLL []byte

var (
	libOnce sync.Once
	libInst *codebookLibrary
)

func library() *codebookLibrary {
	libOnce.Do(func() { libInst = newCodebookLibrary(codebooksData) })
	return libInst
}
