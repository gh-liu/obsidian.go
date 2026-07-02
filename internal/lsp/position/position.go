package position

import "unicode/utf8"

// Encoder converts between UTF-8 byte offsets and LSP character positions.
// LSP clients may use utf-8 or utf-16 encoding for Character values.
type Encoder struct {
	Encoding string // "utf-8" or "utf-16"
}

// ByteToChar converts a UTF-8 byte offset within line to an LSP character offset.
func (e Encoder) ByteToChar(line string, byteOff int) int {
	if e.Encoding == "utf-8" {
		return byteOff
	}
	// UTF-16: count code units. BMP chars = 1 unit, supplementary (>U+FFFF) = 2 units.
	charOff := 0
	i := 0
	for i < len(line) && i < byteOff {
		r, size := utf8.DecodeRuneInString(line[i:])
		// If byteOff falls inside this multi-byte character, stop.
		if i+size > byteOff {
			break
		}
		if r >= 0x10000 {
			charOff += 2
		} else {
			charOff++
		}
		i += size
	}
	return charOff
}

// CharToByte converts an LSP character offset to a UTF-8 byte offset within line.
// Returns the byte offset at the start of the character at charOff.
// If charOff falls inside a multi-unit character, returns the start of that character.
// If charOff is past the end, returns len(line).
func (e Encoder) CharToByte(line string, charOff int) int {
	if e.Encoding == "utf-8" {
		return charOff
	}
	byteOff := 0
	charIdx := 0
	for byteOff < len(line) {
		if charIdx >= charOff {
			return byteOff
		}
		r, size := utf8.DecodeRuneInString(line[byteOff:])
		step := 1
		if r >= 0x10000 {
			step = 2
		}
		// If charOff falls inside a supplementary char, return its start.
		if charIdx+step > charOff {
			return byteOff
		}
		charIdx += step
		byteOff += size
	}
	return len(line)
}
