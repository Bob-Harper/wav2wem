// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	"sync"
)

// We have completely eliminated ALL //go:embed statements.
// The codebook bytes are now provided via a standard Go data source file.
var (
	libOnce sync.Once
	libInst *codebookLibrary
)

func library() *codebookLibrary {
	// CodebooksByteData will be defined as a standard native slice in codebooks_data.go
	libOnce.Do(func() { libInst = newCodebookLibrary(CodebooksByteData) })
	return libInst
}
