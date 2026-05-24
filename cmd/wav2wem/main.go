// SPDX-License-Identifier: GPL-2.0-or-later

// Command wav2wem converts .wav/.flac to Wwise Vorbis .wem without Wwise.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pas2k/wav2wem"
)

func main() {
	var (
		output      = flag.String("o", "", "output .wem (single-file mode)")
		indir       = flag.String("indir", "", "batch: input dir (recurses for .wav/.flac)")
		outdir      = flag.String("outdir", "", "batch: output dir (mirrors structure)")
		update      = flag.Bool("update", false, "batch: only (re)encode when source newer/output missing")
		jobs        = flag.Int("jobs", min(8, runtime.NumCPU()), "parallel workers")
		quality     = flag.Float64("q", 4, "oggenc VBR quality -1..10 (default 4 ~= 57 kbps)")
		bitrate     = flag.Int("b", 0, "managed bitrate kbps (overrides -q)")
		resample    = flag.Int("resample", 0, "resample to this Hz")
		downmix     = flag.Bool("downmix", false, "downmix to mono")
		noHash      = flag.Bool("no-hash", false, "omit the 'hash' chunk")
		decodeAlloc = flag.Int("decode-alloc", -1, "override dwDecodeAllocSize")
		x64Alloc    = flag.Int("x64-alloc", -1, "override dwDecodeX64AllocSize")
	)
	// Parse flags that may be interspersed with positional args (so both
	// `wav2wem -o out.wem in.flac` and `wav2wem in.flac -o out.wem` work).
	flag.Parse()
	var positionals []string
	for args := flag.Args(); len(args) > 0; args = flag.Args() {
		positionals = append(positionals, args[0])
		flag.CommandLine.Parse(args[1:])
	}

	enc := wav2wem.EncodeOptions{Quality: *quality, Bitrate: *bitrate, Resample: *resample, Downmix: *downmix}
	opts := wav2wem.Options{IncludeHash: !*noHash}
	if *decodeAlloc >= 0 {
		v := uint32(*decodeAlloc)
		opts.DecodeAlloc = &v
	}
	if *x64Alloc >= 0 {
		v := uint32(*x64Alloc)
		opts.X64Alloc = &v
	}

	if *indir != "" {
		os.Exit(batch(*indir, *outdir, *update, *jobs, enc, opts))
	}
	if len(positionals) < 1 {
		fmt.Fprintln(os.Stderr, "usage: wav2wem <input.wav|.flac> [-o out.wem]   |   wav2wem -indir DIR -outdir DIR")
		os.Exit(2)
	}
	src := positionals[0]
	dst := *output
	if dst == "" {
		dst = strings.TrimSuffix(src, filepath.Ext(src)) + ".wem"
	}
	if err := wav2wem.ConvertFile(src, dst, enc, opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", dst)
}

type job struct{ src, dst, rel string }

func batch(indir, outdir string, update bool, jobs int, enc wav2wem.EncodeOptions, opts wav2wem.Options) int {
	if outdir == "" {
		outdir = indir
	}
	var work []job
	skipped := 0
	filepath.WalkDir(indir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".flac" && ext != ".wav" {
			return nil
		}
		rel, _ := filepath.Rel(indir, path)
		dst := filepath.Join(outdir, strings.TrimSuffix(rel, filepath.Ext(rel))+".wem")
		if update {
			if si, e1 := os.Stat(path); e1 == nil {
				if di, e2 := os.Stat(dst); e2 == nil && !di.ModTime().Before(si.ModTime()) {
					skipped++
					return nil
				}
			}
		}
		work = append(work, job{path, dst, rel})
		return nil
	})

	total := len(work)
	fmt.Printf("Found %d file(s) to convert", total)
	if update {
		fmt.Printf(" (%d up-to-date, skipped)", skipped)
	}
	fmt.Printf("\nWorkers: %d\n\n", jobs)
	if total == 0 {
		return 0
	}

	var done, failed int64
	start := time.Now()
	ch := make(chan job)
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if err := wav2wem.ConvertFile(j.src, j.dst, enc, opts); err != nil {
					atomic.AddInt64(&failed, 1)
					fmt.Fprintf(os.Stderr, "  [FAIL] %s: %v\n", j.rel, err)
				}
				n := atomic.AddInt64(&done, 1)
				if n%50 == 0 || int(n) == total {
					el := time.Since(start)
					rate := float64(n) / el.Seconds()
					fmt.Printf("  %d/%d (%d%%) | %s | %.0f/min\n", n, total, n*100/int64(total),
						el.Round(time.Second), rate*60)
				}
			}
		}()
	}
	for _, j := range work {
		ch <- j
	}
	close(ch)
	wg.Wait()

	el := time.Since(start)
	fmt.Printf("\nDone. ok=%d failed=%d skipped=%d in %s (%.0f files/min)\n",
		done-failed, failed, skipped, el.Round(time.Second), float64(done)/el.Seconds()*60)
	if failed > 0 {
		return 1
	}
	return 0
}
