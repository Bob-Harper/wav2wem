// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

// repackAudioPacket inverts ww2ogg's mod_packets transform for one packet.
// Ogg: [packet_type=0 (1)][mode (mode_bits)][if long: prev_win(1) next_win(1)][audio...]
// Wwise: [mode (mode_bits)][audio...], byte-aligned (size prefix added by caller).
func repackAudioPacket(oggPacket []byte, modeBlockflag []bool, modeBits int) []byte {
	r := newBitReader(oggPacket)
	w := newBitWriter()

	if r.read(1) != 0 {
		failf("audio packet_type != 0")
	}
	mode := uint32(0)
	if modeBits > 0 {
		mode = r.read(modeBits)
	}
	w.write(mode, modeBits)

	if modeBlockflag[mode] {
		r.read(1) // prev window type (recomputed by decoder)
		r.read(1) // next window type
	}

	total := len(oggPacket) * 8
	for r.bitsRead() < total {
		w.putBit(r.getBit())
	}
	w.flushByte()
	return w.bytes()
}
