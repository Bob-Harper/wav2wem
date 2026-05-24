// SPDX-License-Identifier: GPL-2.0-or-later

package wav2wem

import (
	"bytes"
	"encoding/binary"
)

// oggPacket is one Vorbis packet plus the granule of the page that completed it.
type oggPacket struct {
	data    []byte
	granule int64
}

type oggPage struct {
	serial         uint32
	granule        int64
	headerType     byte
	segSizes       []int // packet-segment lengths derived from lacing
	lastIncomplete bool  // final segment was 255 (packet continues next page)
	body           []byte
}

func parsePages(data []byte) []oggPage {
	var pages []oggPage
	pos, n := 0, len(data)
	for pos < n {
		if !bytes.Equal(data[pos:min(pos+4, n)], []byte("OggS")) {
			nxt := bytes.Index(data[min(pos+1, n):], []byte("OggS"))
			if nxt < 0 {
				break
			}
			pos = pos + 1 + nxt
			continue
		}
		if pos+27 > n {
			break
		}
		headerType := data[pos+5]
		granule := int64(binary.LittleEndian.Uint64(data[pos+6 : pos+14]))
		serial := binary.LittleEndian.Uint32(data[pos+14 : pos+18])
		nsegs := int(data[pos+26])
		if pos+27+nsegs > n {
			break
		}
		segTable := data[pos+27 : pos+27+nsegs]
		bodyStart := pos + 27 + nsegs
		var segSizes []int
		cur := 0
		lastIncomplete := false
		bodyLen := 0
		for i, lv := range segTable {
			cur += int(lv)
			bodyLen += int(lv)
			if lv < 255 {
				segSizes = append(segSizes, cur)
				cur = 0
			} else if i == nsegs-1 {
				segSizes = append(segSizes, cur)
				lastIncomplete = true
			}
		}
		if bodyStart+bodyLen > n {
			break
		}
		pages = append(pages, oggPage{serial, granule, headerType, segSizes, lastIncomplete,
			data[bodyStart : bodyStart+bodyLen]})
		pos = bodyStart + bodyLen
	}
	return pages
}

// demuxPackets returns the Vorbis packets of the first logical stream,
// reassembling packets that span pages.
func demuxPackets(data []byte) []oggPacket {
	var packets []oggPacket
	var carry []byte
	carrying := false
	serial := uint32(0)
	haveSerial := false
	for _, page := range parsePages(data) {
		if !haveSerial {
			serial = page.serial
			haveSerial = true
		}
		if page.serial != serial {
			continue
		}
		off := 0
		for idx, sz := range page.segSizes {
			chunk := page.body[off : off+sz]
			off += sz
			isLast := idx == len(page.segSizes)-1
			if carrying {
				carry = append(carry, chunk...)
				if isLast && page.lastIncomplete {
					// still continuing
				} else {
					packets = append(packets, oggPacket{append([]byte(nil), carry...), page.granule})
					carry = carry[:0]
					carrying = false
				}
			} else {
				if isLast && page.lastIncomplete {
					carry = append(carry[:0], chunk...)
					carrying = true
				} else {
					packets = append(packets, oggPacket{append([]byte(nil), chunk...), page.granule})
				}
			}
		}
	}
	if carrying && len(carry) > 0 {
		packets = append(packets, oggPacket{append([]byte(nil), carry...), -1})
	}
	return packets
}
