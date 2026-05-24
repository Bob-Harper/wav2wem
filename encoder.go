// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	"crypto/sha256"
	"encoding/hex"
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

// encoderPath extracts the embedded oggenc2.exe + libFLAC.dll to a per-version
// temp dir (so the DLL sits beside the exe) and returns the exe path.
func encoderPath() (string, error) {
	encOnce.Do(func() {
		sum := sha256.Sum256(oggencExe)
		dir := filepath.Join(os.TempDir(), "wav2wem-enc-"+hex.EncodeToString(sum[:6]))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			encErr = err
			return
		}
		exe := filepath.Join(dir, "oggenc2.exe")
		dll := filepath.Join(dir, "libFLAC.dll")
		if err := writeIfMissing(exe, oggencExe); err != nil {
			encErr = err
			return
		}
		if err := writeIfMissing(dll, libflacDLL); err != nil {
			encErr = err
			return
		}
		encPath = exe
	})
	return encPath, encErr
}

func writeIfMissing(path string, data []byte) error {
	if fi, err := os.Stat(path); err == nil && fi.Size() == int64(len(data)) {
		return nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// encodeToOgg encodes a .wav/.flac source to Ogg Vorbis bytes via oggenc2.
func encodeToOgg(src string, enc EncodeOptions) ([]byte, error) {
	exe, err := encoderPath()
	if err != nil {
		return nil, fmt.Errorf("prepare encoder: %w", err)
	}
	tmp, err := os.CreateTemp("", "wav2wem-*.ogg")
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
