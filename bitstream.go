// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

// LSB-first bit reader/writer matching ww2ogg's Bit_stream / Bit_oggstream and
// the Python reference (src/bitstream.py). Vorbis packs bits LSB-first within
// each byte: the first bit read/written is bit 0 of the byte; an n-bit value is
// value == sum(bit_i << i) with bit_0 first.

// errOutOfBits is raised (panicked) by BitReader when the stream is exhausted;
// BuildWem recovers it into a normal error.
type errOutOfBits struct{}

type bitReader struct {
	data    []byte
	bytePos int
	buf     byte
	left    int
	total   int
}

func newBitReader(data []byte) *bitReader { return &bitReader{data: data} }

func (r *bitReader) getBit() uint32 {
	if r.left == 0 {
		if r.bytePos >= len(r.data) {
			panic(errOutOfBits{})
		}
		r.buf = r.data[r.bytePos]
		r.bytePos++
		r.left = 8
	}
	bit := uint32(r.buf>>(8-uint(r.left))) & 1
	r.left--
	r.total++
	return bit
}

func (r *bitReader) read(n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		v |= r.getBit() << uint(i)
	}
	return v
}

func (r *bitReader) bitsRead() int { return r.total }

type bitWriter struct {
	out    []byte
	buf    byte
	stored int
	total  int
}

func newBitWriter() *bitWriter { return &bitWriter{} }

func (w *bitWriter) putBit(bit uint32) {
	if bit&1 != 0 {
		w.buf |= 1 << uint(w.stored)
	}
	w.stored++
	w.total++
	if w.stored == 8 {
		w.out = append(w.out, w.buf)
		w.buf = 0
		w.stored = 0
	}
}

func (w *bitWriter) write(value uint32, n int) {
	for i := 0; i < n; i++ {
		w.putBit((value >> uint(i)) & 1)
	}
}

// flushByte zero-pads the current partial byte to a byte boundary.
func (w *bitWriter) flushByte() {
	if w.stored != 0 {
		w.out = append(w.out, w.buf)
		w.buf = 0
		w.stored = 0
	}
}

func (w *bitWriter) bytes() []byte { return w.out }
