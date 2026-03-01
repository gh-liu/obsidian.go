package lsp

import (
	"unicode/utf16"
)

// PositionEncoder converts between LSP character offset and UTF-8 byte offset.
// LSP clients may use utf-8 or utf-16 for Position.Character.
type PositionEncoder struct {
	encoding string // "utf-8" or "utf-16"
}

// CharToByte converts LSP character offset to UTF-8 byte offset in line.
func (e PositionEncoder) CharToByte(line string, charOff int) int {
	if e.encoding == "utf-8" {
		if charOff > len(line) {
			return len(line)
		}
		return charOff
	}
	var n int
	for i, r := range line {
		if n >= charOff {
			return i
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return len(line)
}

// ByteToChar converts UTF-8 byte offset to LSP character offset in line.
func (e PositionEncoder) ByteToChar(line string, byteOff int) int {
	if e.encoding == "utf-8" {
		if byteOff > len(line) {
			return len(line)
		}
		return byteOff
	}
	var n int
	for i, r := range line {
		if i >= byteOff {
			return n
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return n
}
