// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// EncodeOptions controls the aoTuV oggenc2 encode step.
type EncodeOptions struct {
	Quality  float64 // VBR quality -1..10 (used when Bitrate == 0)
	Bitrate  int     // managed bitrate kbps (0 = use Quality)
	Resample int     // resample Hz (0 = none)
	Downmix  bool    // downmix to mono
}

// DefaultEncodeOptions matches the reference corpus (~57 kbps mono 24k).
func DefaultEncodeOptions() EncodeOptions { return EncodeOptions{Quality: 4} }

var (
	encOnce sync.Once
	encPath string
	encErr  error
)

// encoderPath now cleanly locates the asset on disk right beside the running binary.
// No writing to the Windows Temp directory, no runtime binary dropping!
func encoderPath() (string, error) {
	encOnce.Do(func() {
		// Look up the path of the currently executing wav2wem.exe
		execPath, err := os.Executable()
		if err != nil {
			encErr = fmt.Errorf("failed to look up executable path: %w", err)
			return
		}
		execDir := filepath.Dir(execPath)

		// Point directly to the assets directory shipped with your mod pack
		exeTarget := filepath.Join(execDir, "assets", "oggenc2.exe")

		// Verify the binary actually exists where it's supposed to be
		if _, err := os.Stat(exeTarget); os.IsNotExist(err) {
			encErr = fmt.Errorf("standalone encoder binary missing at target path: %s", exeTarget)
			return
		}

		encPath = exeTarget
	})
	return encPath, encErr
}

// encodeToOgg encodes a .wav/.flac source to Ogg Vorbis bytes via oggenc2.
func encodeToOgg(src string, enc EncodeOptions) ([]byte, error) {
	exe, err := encoderPath()
	if err != nil {
		return nil, fmt.Errorf("prepare encoder: %w", err)
	}

	// Staging target: Keep the temporary .ogg buffer contained inside the project workspace directory
	execPath, _ := os.Executable()
	workspaceDir := filepath.Dir(execPath)

	tmp, err := os.CreateTemp(workspaceDir, "w2w-stream-*.ogg")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	args := []string{src, "--quiet", "-o", tmp.Name()}
	if enc.Bitrate > 0 {
		args = append(args, "-b", fmt.Sprint(enc.Bitrate), "--managed")
	} else {
		args = append(args, "-q", fmt.Sprintf("%g", enc.Quality))
	}
	if enc.Resample > 0 {
		args = append(args, "--resample", fmt.Sprint(enc.Resample))
	}
	if enc.Downmix {
		args = append(args, "--downmix")
	}

	cmd := exec.Command(exe, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("oggenc2 failed: %v: %s", err, out)
	}
	return os.ReadFile(tmp.Name())
}

// ConvertToWem encodes src (.wav/.flac) and returns Wwise .wem bytes. If src is
// already an .ogg, it is used directly (no re-encode) — useful for testing.
func ConvertToWem(src string, enc EncodeOptions, opts Options) ([]byte, error) {
	var ogg []byte
	var err error
	if strings.EqualFold(filepath.Ext(src), ".ogg") {
		ogg, err = os.ReadFile(src)
	} else {
		ogg, err = encodeToOgg(src, enc)
	}
	if err != nil {
		return nil, err
	}
	return BuildWem(ogg, opts)
}

// ConvertFile converts src to a .wem written at dst.
func ConvertFile(src, dst string, enc EncodeOptions, opts Options) error {
	wem, err := ConvertToWem(src, enc, opts)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, wem, 0o644)
}
