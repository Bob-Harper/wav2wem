# wav2wem

Convert `.wav` / `.flac` to **Wwise Vorbis `.wem`** — without Wwise. A single
self-contained Go binary: the aoTuV-6.03 encoder and the codebook library are
embedded (`//go:embed`), so there's nothing else to install.

```
go install github.com/pas2k/wav2wem/cmd/wav2wem@latest
```

## How it works

```
PCM (wav/flac)
  └─ oggenc2 (libvorbis 1.3.7 + aoTuV b6.03, embedded)  ──>  Ogg Vorbis
       └─ strip setup + repacketize + RIFF              ──>  .wem
```

It is the inverse of [`ww2ogg`](https://github.com/hcs64/ww2ogg). Modern Wwise's
built-in Vorbis codebooks *are* the aoTuV 6.03 static codebooks, so the encoder's
full codebooks are replaced by 10-bit indices into the embedded library
(`packed_codebooks_aoTuV_603.bin`); the setup header is stripped and audio
packets are reframed into Wwise's `mod_packets` form. Output targets the modern
variant: `fmt` 0x42 with embedded vorb fields, no-granule 2-byte packets,
mod_packets, stripped setup, external aoTuV-6.03 codebooks, and a seek table.

## CLI

```sh
wav2wem voice.flac                       # -> voice.wem
wav2wem in.wav -o out.wem -q 2
wav2wem -indir ./flac -outdir ./wem            # batch, mirrors structure
wav2wem -indir ./flac -outdir ./wem -update -jobs 12
```

Flags: `-q` VBR quality (default 4 ≈ 57 kbps), `-b` managed kbps, `-resample`,
`-downmix`, `-no-hash`, `-decode-alloc`/`-x64-alloc`, `-jobs`. An `.ogg` input is
used as-is (no re-encode).

## Library

```go
import "github.com/pas2k/wav2wem"

// from a source file:
err := wav2wem.ConvertFile("in.flac", "out.wem",
    wav2wem.DefaultEncodeOptions(), wav2wem.Options{IncludeHash: true})

// or from Ogg Vorbis bytes you already have:
wem, err := wav2wem.BuildWem(oggBytes, wav2wem.Options{IncludeHash: true})
```

## Correctness

`BuildWem` is a bit-exact port of a Python reference implementation; outputs are
**byte-for-byte identical** to it across a 112-file corpus (both per-Ogg and
full pipeline). Outputs decode in vgmstream/ww2ogg with the correct sample
counts, matching real Wwise output. `go test` runs a committed golden case.

Field values (seek table, `uLastGranuleExtra`, `subtype`, `uHashCodebook`,
alloc sizes) were reverse-engineered from the corpus; for the mono/24 kHz config
the output is structurally equivalent to real Wwise files. The only field not
reproduced is the 16-byte `hash` chunk's contents (emitted zeroed; RIFF parsers
skip it and it appears to be an authoring artifact). See the parent research
project for the full format write-up.

## Licensing

**GPLv2** (see [LICENSE](LICENSE)). The project embeds `oggenc2.exe` (a GPLv2
build of vorbis-tools), so the whole project is GPLv2 — simple and unambiguous.
Embedded BSD assets (the aoTuV codebook library, `libFLAC.dll`) are
GPL-compatible. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for
attributions and the GPLv2 §3 source offers.
