// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import "fmt"

// parseError is panicked by the setup stripper and recovered by BuildWem.
type parseError struct{ msg string }

func failf(format string, a ...interface{}) { panic(parseError{fmt.Sprintf(format, a...)}) }

type idHeader struct {
	channels       uint32
	sampleRate     uint32
	bitrateNominal uint32
	blocksize0     uint32 // short block exponent
	blocksize1     uint32 // long block exponent
}

func parseIDHeader(packet []byte) idHeader {
	if len(packet) < 7 || packet[0] != 1 || string(packet[1:7]) != "vorbis" {
		failf("not an id header")
	}
	r := newBitReader(packet[7:])
	r.read(32) // version
	ch := r.read(8)
	rate := r.read(32)
	r.read(32) // bitrate max
	brnom := r.read(32)
	r.read(32) // bitrate min
	bs0 := r.read(4)
	bs1 := r.read(4)
	return idHeader{ch, rate, brnom, bs0, bs1}
}

// stripSetup reads a FULL Vorbis setup header and writes Wwise's STRIPPED form
// (codebooks -> 10-bit ids; constant 16-bit type fields, the time-domain
// transform placeholder, and the trailing framing bit dropped; residue_type
// squeezed 16->2 bits). Returns stripped bytes, mode block flags, mode bits.
func stripSetup(packet []byte, channels uint32, lib *codebookLibrary) ([]byte, []bool, int) {
	if len(packet) < 7 || packet[0] != 5 || string(packet[1:7]) != "vorbis" {
		failf("not a setup header")
	}
	r := newBitReader(packet[7:])
	w := newBitWriter()

	// codebooks
	codebookCount := r.read(8) + 1
	w.write(codebookCount-1, 8)
	for i := uint32(0); i < codebookCount; i++ {
		c, err := parseFull(r)
		if err != nil {
			failf("codebook %d: %v", i, err)
		}
		id, err := lib.matchID(c)
		if err != nil {
			failf("codebook %d: %v", i, err)
		}
		w.write(uint32(id), 10)
	}

	// time domain transforms (placeholder in full, absent in stripped)
	timeCount := r.read(6) + 1
	for i := uint32(0); i < timeCount; i++ {
		if r.read(16) != 0 {
			failf("nonzero time domain transform")
		}
	}

	// floors
	floorCount := r.read(6) + 1
	w.write(floorCount-1, 6)
	for i := uint32(0); i < floorCount; i++ {
		if r.read(16) != 1 {
			failf("unexpected floor type")
		}
		partitions := r.read(5)
		w.write(partitions, 5)
		partitionClass := make([]uint32, partitions)
		maxClass := uint32(0)
		for j := uint32(0); j < partitions; j++ {
			pc := r.read(4)
			w.write(pc, 4)
			partitionClass[j] = pc
			if pc > maxClass {
				maxClass = pc
			}
		}
		classDims := make([]uint32, maxClass+1)
		for j := uint32(0); j <= maxClass; j++ {
			dimsLess1 := r.read(3)
			w.write(dimsLess1, 3)
			classDims[j] = dimsLess1 + 1
			subclasses := r.read(2)
			w.write(subclasses, 2)
			if subclasses != 0 {
				w.write(r.read(8), 8) // masterbook
			}
			for k := uint32(0); k < (1 << subclasses); k++ {
				w.write(r.read(8), 8) // subclass_book_plus1
			}
		}
		w.write(r.read(2), 2) // multiplier_less1
		rangebits := r.read(4)
		w.write(rangebits, 4)
		for j := uint32(0); j < partitions; j++ {
			for k := uint32(0); k < classDims[partitionClass[j]]; k++ {
				w.write(r.read(int(rangebits)), int(rangebits))
			}
		}
	}

	// residues
	residueCount := r.read(6) + 1
	w.write(residueCount-1, 6)
	for i := uint32(0); i < residueCount; i++ {
		residueType := r.read(16) // full: 16 bits
		if residueType > 2 {
			failf("invalid residue type")
		}
		w.write(residueType, 2) // stripped: 2 bits
		w.write(r.read(24), 24) // begin
		w.write(r.read(24), 24) // end
		w.write(r.read(24), 24) // partition_size_less1
		classifications := r.read(6) + 1
		w.write(classifications-1, 6)
		w.write(r.read(8), 8) // classbook
		cascade := make([]uint32, classifications)
		for j := uint32(0); j < classifications; j++ {
			low := r.read(3)
			w.write(low, 3)
			bitflag := r.read(1)
			w.write(bitflag, 1)
			high := uint32(0)
			if bitflag != 0 {
				high = r.read(5)
				w.write(high, 5)
			}
			cascade[j] = high*8 + low
		}
		for j := uint32(0); j < classifications; j++ {
			for k := uint32(0); k < 8; k++ {
				if cascade[j]&(1<<k) != 0 {
					w.write(r.read(8), 8) // residue_book
				}
			}
		}
	}

	// mappings
	mappingCount := r.read(6) + 1
	w.write(mappingCount-1, 6)
	couplingBits := ilog(channels - 1)
	for i := uint32(0); i < mappingCount; i++ {
		if r.read(16) != 0 { // mapping_type, full: 16 bits
			failf("unexpected mapping type")
		}
		submapsFlag := r.read(1)
		w.write(submapsFlag, 1)
		submaps := uint32(1)
		if submapsFlag != 0 {
			submapsLess1 := r.read(4)
			w.write(submapsLess1, 4)
			submaps = submapsLess1 + 1
		}
		squarePolar := r.read(1)
		w.write(squarePolar, 1)
		if squarePolar != 0 {
			couplingSteps := r.read(8) + 1
			w.write(couplingSteps-1, 8)
			for j := uint32(0); j < couplingSteps; j++ {
				w.write(r.read(couplingBits), couplingBits) // magnitude
				w.write(r.read(couplingBits), couplingBits) // angle
			}
		}
		mappingReserved := r.read(2)
		w.write(mappingReserved, 2) // Wwise keeps this field (ww2ogg copies it through)
		if mappingReserved != 0 {
			failf("mapping reserved nonzero")
		}
		if submaps > 1 {
			for j := uint32(0); j < channels; j++ {
				w.write(r.read(4), 4) // mapping_mux
			}
		}
		for j := uint32(0); j < submaps; j++ {
			w.write(r.read(8), 8) // time_config
			w.write(r.read(8), 8) // floor_number
			w.write(r.read(8), 8) // residue_number
		}
	}

	// modes
	modeCount := r.read(6) + 1
	w.write(modeCount-1, 6)
	modeBlockflag := make([]bool, modeCount)
	modeBits := ilog(modeCount - 1)
	for i := uint32(0); i < modeCount; i++ {
		blockFlag := r.read(1)
		w.write(blockFlag, 1)
		modeBlockflag[i] = blockFlag != 0
		if r.read(16) != 0 || r.read(16) != 0 { // windowtype, transformtype
			failf("nonzero window/transform type")
		}
		w.write(r.read(8), 8) // mapping
	}

	if r.read(1) != 1 { // framing bit (full only)
		failf("missing framing bit")
	}

	w.flushByte()
	return w.bytes(), modeBlockflag, modeBits
}
