// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	"encoding/binary"
	"fmt"
)

const seekInterval = 16384 // Wwise emits a seek point every >=16384 output samples

// Constants observed across the whole mono/24k/aoTuV-6.03 corpus.
const uidAoTuV603 = 0x20CEF588 // uHashCodebook: hash of the aoTuV-6.03 setup/codebooks

// Options controls per-file WEM fields. Zero value uses corpus-faithful defaults.
type Options struct {
	IncludeHash bool   // include a (zeroed) "hash" chunk (default true via Convert*)
	HashBytes   []byte // optional 16-byte hash contents
	DecodeAlloc *uint32
	X64Alloc    *uint32
	Subtype     *uint32
	UID         *uint32
}

func subtypeForChannels(ch uint32) uint32 {
	if ch == 1 {
		return 0x4101 // only mono confirmed
	}
	return 0
}

func estimateDecodeAlloc(channels, blocksize1Exp uint32) (uint32, uint32) {
	scale := float64(channels) * float64(uint32(1)<<blocksize1Exp) / 1024.0
	return uint32(17692*scale + 0.5), uint32(18116*scale + 0.5)
}

func u16(v uint32) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}
func u32(v uint32) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }

// BuildWem assembles a Wwise Vorbis .wem from aoTuV ogg bytes. Mirrors the
// Python reference (src/wem.py) byte-for-byte.
func BuildWem(oggBytes []byte, opts Options) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case parseError:
				err = fmt.Errorf("%s", e.msg)
			case errOutOfBits:
				err = fmt.Errorf("vorbis bitstream truncated")
			default:
				panic(r)
			}
		}
	}()

	lib := library()
	packets := demuxPackets(oggBytes)
	if len(packets) < 4 {
		return nil, fmt.Errorf("too few ogg packets")
	}

	idh := parseIDHeader(packets[0].data)
	channels := idh.channels
	setupBytes, modeBlockflag, modeBits := stripSetup(packets[2].data, channels, lib)

	audioPackets := packets[3:]
	var sampleCount int64
	for _, p := range audioPackets {
		if p.granule >= 0 && p.granule > sampleCount {
			sampleCount = p.granule
		}
	}

	shortBs := uint32(1) << idh.blocksize0
	longBs := uint32(1) << idh.blocksize1
	setupChunk := append(u16(uint32(len(setupBytes))), setupBytes...)
	setupChunkLen := len(setupChunk) // = dwLoopStartPacketOffset

	var audioBlob []byte
	var seekTable []byte
	maxPacket := uint32(0)
	var prevBs int64 = -1
	var cum, lastSample, lastOffset int64
	for _, p := range audioPackets {
		payload := repackAudioPacket(p.data, modeBlockflag, modeBits)
		b0 := p.data[0]
		mode := uint32(0)
		if modeBits > 0 {
			mode = uint32(b0>>1) & ((1 << uint(modeBits)) - 1)
		}
		curBs := int64(shortBs)
		if modeBlockflag[mode] {
			curBs = int64(longBs)
		}
		if prevBs >= 0 {
			cum += (prevBs + curBs) / 4
		}
		prevBs = curBs

		pktOffset := int64(setupChunkLen + len(audioBlob))
		if cum-lastSample >= seekInterval {
			seekTable = append(seekTable, u16(uint32(cum-lastSample))...)
			seekTable = append(seekTable, u16(uint32(pktOffset-lastOffset))...)
			lastSample = cum
			lastOffset = pktOffset
		}
		if uint32(len(payload)) > maxPacket {
			maxPacket = uint32(len(payload))
		}
		audioBlob = append(audioBlob, u16(uint32(len(payload)))...)
		audioBlob = append(audioBlob, payload...)
	}

	gFull := cum
	lastGranuleExtra := gFull - sampleCount
	if lastGranuleExtra < 0 {
		lastGranuleExtra = 0
	}

	data := append(append(append([]byte{}, seekTable...), setupChunk...), audioBlob...)
	seekSize := len(seekTable)
	audioOffset := seekSize + setupChunkLen // dwVorbisDataOffset
	dataSize := len(data)

	avgBytes := uint32(0)
	if idh.bitrateNominal != 0 {
		avgBytes = idh.bitrateNominal / 8
	}
	decodeAlloc, x64Alloc := estimateDecodeAlloc(channels, idh.blocksize1)
	if opts.DecodeAlloc != nil {
		decodeAlloc = *opts.DecodeAlloc
	}
	if opts.X64Alloc != nil {
		x64Alloc = *opts.X64Alloc
	}
	subtype := subtypeForChannels(channels)
	if opts.Subtype != nil {
		subtype = *opts.Subtype
	}
	uid := uint32(uidAoTuV603)
	if opts.UID != nil {
		uid = *opts.UID
	}

	// fmt chunk (0x42 bytes)
	var fmtc []byte
	fmtc = append(fmtc, u16(0xFFFF)...)                             // 0x00 codec id
	fmtc = append(fmtc, u16(channels)...)                           // 0x02
	fmtc = append(fmtc, u32(idh.sampleRate)...)                     // 0x04
	fmtc = append(fmtc, u32(avgBytes)...)                           // 0x08
	fmtc = append(fmtc, u16(0)...)                                  // 0x0C block align
	fmtc = append(fmtc, u16(0)...)                                  // 0x0E bits/sample
	fmtc = append(fmtc, u16(0x30)...)                               // 0x10 extra size
	fmtc = append(fmtc, u16(0)...)                                  // 0x12 ext_unk
	fmtc = append(fmtc, u32(subtype)...)                            // 0x14 subtype/channel cfg
	fmtc = append(fmtc, u32(uint32(sampleCount))...)                // 0x18 dwTotalPCMFrames
	fmtc = append(fmtc, u32(uint32(setupChunkLen))...)              // 0x1c dwLoopStartPacketOffset
	fmtc = append(fmtc, u32(uint32(dataSize-seekSize))...)          // 0x20 dwLoopEndPacketOffset
	fmtc = append(fmtc, u16(0)...)                                  // 0x24 uLoopBeginExtra
	fmtc = append(fmtc, u16(uint32(lastGranuleExtra))...)           // 0x26 uLoopEndExtra
	fmtc = append(fmtc, u32(uint32(seekSize))...)                   // 0x28 dwSeekTableSize
	fmtc = append(fmtc, u32(uint32(audioOffset))...)                // 0x2c dwVorbisDataOffset
	fmtc = append(fmtc, u16(maxPacket)...)                          // 0x30 uMaxPacketSize
	fmtc = append(fmtc, u16(uint32(lastGranuleExtra))...)           // 0x32 uLastGranuleExtra
	fmtc = append(fmtc, u32(decodeAlloc)...)                        // 0x34 dwDecodeAllocSize
	fmtc = append(fmtc, u32(x64Alloc)...)                           // 0x38 dwDecodeX64AllocSize
	fmtc = append(fmtc, u32(uid)...)                                // 0x3c uHashCodebook/uid
	fmtc = append(fmtc, byte(idh.blocksize0), byte(idh.blocksize1)) // 0x40/0x41
	if len(fmtc) != 0x42 {
		return nil, fmt.Errorf("fmt size %d != 0x42", len(fmtc))
	}

	chunk := func(tag string, body []byte) []byte {
		c := append([]byte(tag), u32(uint32(len(body)))...)
		c = append(c, body...)
		if len(body)&1 == 1 {
			c = append(c, 0)
		}
		return c
	}

	body := []byte("WAVE")
	body = append(body, chunk("fmt ", fmtc)...)
	if opts.IncludeHash {
		h := make([]byte, 16)
		copy(h, opts.HashBytes)
		body = append(body, chunk("hash", h)...)
	}
	body = append(body, chunk("data", data)...)

	out = append([]byte("RIFF"), u32(uint32(len(body)))...)
	out = append(out, body...)
	return out, nil
}
