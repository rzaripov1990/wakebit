package tray

import (
	"bytes"
	"encoding/binary"
)

// encodeIcon wraps a PNG in a single-image ICO container (Vista+ supports PNG-in-ICO).
func encodeIcon(pngData []byte) []byte {
	var b bytes.Buffer
	b.Write([]byte{0, 0, 1, 0, 1, 0}) // ICONDIR: reserved, type=icon, count=1
	b.Write([]byte{iconSize, iconSize, 0, 0})
	binary.Write(&b, binary.LittleEndian, uint16(1))  // planes
	binary.Write(&b, binary.LittleEndian, uint16(32)) // bit count
	binary.Write(&b, binary.LittleEndian, uint32(len(pngData)))
	binary.Write(&b, binary.LittleEndian, uint32(22)) // data offset: 6 + 16
	b.Write(pngData)
	return b.Bytes()
}
