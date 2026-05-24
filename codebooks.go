// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func ilog(v uint32) int {
	ret := 0
	for v != 0 {
		ret++
		v >>= 1
	}
	return ret
}

func bookMaptype1Quantvals(entries, dimensions uint32) uint32 {
	bits := ilog(entries)
	vals := entries >> uint((bits-1)*(int(dimensions)-1)/int(dimensions))
	for {
		var acc, acc1 uint64 = 1, 1
		for i := uint32(0); i < dimensions; i++ {
			acc *= uint64(vals)
			acc1 *= uint64(vals + 1)
		}
		if acc <= uint64(entries) && acc1 > uint64(entries) {
			return vals
		}
		if acc > uint64(entries) {
			vals--
		} else {
			vals++
		}
	}
}

// codebook is the canonical content of a Vorbis codebook (encoding-independent).
type codebook struct {
	dimensions uint32
	entries    uint32
	ordered    bool
	initialLen uint32   // ordered only
	numbers    []uint32 // ordered only
	lengths    []int32  // unordered only; -1 == absent (sparse)
	lookupType uint32
	min, max   uint32
	valueLen   uint32
	seqFlag    uint32
	quantvals  []uint32
}

// key is a deterministic serialization used for matching; identical books
// (whether parsed from compact or full form) produce identical keys.
func (c *codebook) key() string {
	var b bytes.Buffer
	put := func(v uint32) {
		var t [4]byte
		binary.LittleEndian.PutUint32(t[:], v)
		b.Write(t[:])
	}
	put(c.dimensions)
	put(c.entries)
	if c.ordered {
		b.WriteByte(1)
		put(c.initialLen)
		put(uint32(len(c.numbers)))
		for _, n := range c.numbers {
			put(n)
		}
	} else {
		b.WriteByte(0)
		put(uint32(len(c.lengths)))
		for _, l := range c.lengths {
			put(uint32(l)) // -1 -> 0xFFFFFFFF sentinel
		}
	}
	put(c.lookupType)
	if c.lookupType == 1 {
		put(c.min)
		put(c.max)
		put(c.valueLen)
		put(c.seqFlag)
		put(uint32(len(c.quantvals)))
		for _, v := range c.quantvals {
			put(v)
		}
	}
	return b.String()
}

func readOrderedLengths(r *bitReader, entries uint32) (uint32, []uint32) {
	initial := r.read(5)
	var numbers []uint32
	current := uint32(0)
	for current < entries {
		num := r.read(ilog(entries - current))
		numbers = append(numbers, num)
		current += num
	}
	return initial, numbers
}

func readLookup(r *bitReader, c *codebook) {
	if c.lookupType == 0 {
		return
	}
	if c.lookupType == 1 {
		c.min = r.read(32)
		c.max = r.read(32)
		c.valueLen = r.read(4) // stored as length-1
		c.seqFlag = r.read(1)
		q := bookMaptype1Quantvals(c.entries, c.dimensions)
		c.quantvals = make([]uint32, q)
		for i := range c.quantvals {
			c.quantvals[i] = r.read(int(c.valueLen) + 1)
		}
		return
	}
	panic(fmt.Sprintf("unsupported lookup type %d", c.lookupType))
}

// parseFull reads one full inline codebook (as in a Vorbis setup header).
func parseFull(r *bitReader) (*codebook, error) {
	ident := r.read(24)
	if ident != 0x564342 {
		return nil, fmt.Errorf("bad codebook identifier 0x%x", ident)
	}
	c := &codebook{dimensions: r.read(16), entries: r.read(24)}
	if r.read(1) != 0 {
		c.ordered = true
		c.initialLen, c.numbers = readOrderedLengths(r, c.entries)
	} else {
		sparse := r.read(1)
		c.lengths = make([]int32, c.entries)
		for i := uint32(0); i < c.entries; i++ {
			present := true
			if sparse != 0 {
				present = r.read(1) != 0
			}
			if present {
				c.lengths[i] = int32(r.read(5))
			} else {
				c.lengths[i] = -1
			}
		}
	}
	c.lookupType = r.read(4)
	readLookup(r, c)
	return c, nil
}

// parseCompact reads one compact (library) codebook.
func parseCompact(r *bitReader) (*codebook, error) {
	c := &codebook{dimensions: r.read(4), entries: r.read(14)}
	if r.read(1) != 0 {
		c.ordered = true
		c.initialLen, c.numbers = readOrderedLengths(r, c.entries)
	} else {
		cll := r.read(3)
		sparse := r.read(1)
		if cll == 0 || cll > 5 {
			return nil, fmt.Errorf("nonsense codeword length %d", cll)
		}
		c.lengths = make([]int32, c.entries)
		for i := uint32(0); i < c.entries; i++ {
			present := true
			if sparse != 0 {
				present = r.read(1) != 0
			}
			if present {
				c.lengths[i] = int32(r.read(int(cll)))
			} else {
				c.lengths[i] = -1
			}
		}
	}
	c.lookupType = r.read(1)
	readLookup(r, c)
	return c, nil
}

// codebookLibrary maps each compact library codebook to its 10-bit id.
type codebookLibrary struct {
	byKey map[string]int
}

func newCodebookLibrary(data []byte) *codebookLibrary {
	offsetOffset := int(binary.LittleEndian.Uint32(data[len(data)-4:]))
	count := (len(data) - offsetOffset) / 4
	offsets := make([]int, count)
	for i := 0; i < count; i++ {
		offsets[i] = int(binary.LittleEndian.Uint32(data[offsetOffset+4*i : offsetOffset+4*i+4]))
	}
	lib := &codebookLibrary{byKey: make(map[string]int)}
	for i := 0; i < count-1; i++ { // last offset is a sentinel
		cb := data[offsets[i]:offsets[i+1]]
		c, err := safeParseCompact(cb)
		if err != nil || c == nil {
			continue
		}
		k := c.key()
		if _, ok := lib.byKey[k]; !ok { // first id wins (matches Python setdefault)
			lib.byKey[k] = i
		}
	}
	return lib
}

func safeParseCompact(cb []byte) (c *codebook, err error) {
	defer func() {
		if recover() != nil {
			c, err = nil, fmt.Errorf("parse panic")
		}
	}()
	return parseCompact(newBitReader(cb))
}

func (lib *codebookLibrary) matchID(c *codebook) (int, error) {
	if id, ok := lib.byKey[c.key()]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("codebook not found in library")
}
