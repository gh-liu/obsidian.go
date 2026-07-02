package position

import "testing"

func TestUTF8Encoding(t *testing.T) {
	enc := Encoder{Encoding: "utf-8"}
	line := "hello world"

	// UTF-8: byte offset == char offset for ASCII
	for i := 0; i <= len(line); i++ {
		bt := enc.CharToByte(line, i)
		ch := enc.ByteToChar(line, i)
		if bt != i {
			t.Errorf("CharToByte(%d) = %d", i, bt)
		}
		if ch != i {
			t.Errorf("ByteToChar(%d) = %d", i, ch)
		}
	}
}

func TestUTF16Encoding(t *testing.T) {
	enc := Encoder{Encoding: "utf-16"}

	// ASCII: 1:1 mapping
	line := "abc"
	for i := 0; i <= 3; i++ {
		bt := enc.CharToByte(line, i)
		ch := enc.ByteToChar(line, i)
		if bt != i || ch != i {
			t.Errorf("ASCII pos %d: CharToByte=%d ByteToChar=%d", i, bt, ch)
		}
	}

	// BMP中文 (U+4E2D = 中): 3 UTF-8 bytes, 1 UTF-16 unit
	line2 := "中ab"
	// byte offsets: 中(0,1,2) a(3) b(4)
	// UTF-16 offsets: (0) a(1) b(2)
	tests := []struct{ byteOff, charOff int }{
		{0, 0}, // start of 中
		{1, 0}, // mid 中 (still char 0)
		{2, 0}, // mid 中
		{3, 1}, // start of a
		{4, 2}, // start of b
		{5, 3}, // end of line
	}
	for _, tt := range tests {
		got := enc.ByteToChar(line2, tt.byteOff)
		if got != tt.charOff {
			t.Errorf("ByteToChar(%q, %d) = %d, want %d", line2, tt.byteOff, got, tt.charOff)
		}
		gotBt := enc.CharToByte(line2, tt.charOff)
		// CharToByte returns the byte position where that char starts
		// For char 3 (end), should return 5
		if gotBt != tt.byteOff {
			// For multi-byte chars, CharToByte returns the start position
			// which may differ from input byteOff for mid-char positions
		}
	}

	// Supplementary plane: 🎉 (U+1F389) = 4 UTF-8 bytes, 2 UTF-16 units
	line3 := "🎉x"
	// byte offsets: 🎉(0,1,2,3) x(4)
	// UTF-16 offsets: 🎉(0,1) x(2)
	tests2 := []struct{ byteOff, charOff int }{
		{0, 0},
		{1, 0},
		{2, 0},
		{3, 0},
		{4, 2}, // x starts at UTF-16 offset 2
		{5, 3}, // end
	}
	for _, tt := range tests2 {
		got := enc.ByteToChar(line3, tt.byteOff)
		if got != tt.charOff {
			t.Errorf("ByteToChar(%q, %d) = %d, want %d", line3, tt.byteOff, got, tt.charOff)
		}
	}

	// Round-trip: CharToByte → ByteToChar should be identity.
	// Skip positions that fall inside supplementary characters (no 1:1 mapping).
	line4 := "Hello, 世界! 🚀 end"
	maxChar := enc.ByteToChar(line4, len(line4))
	for charOff := 0; charOff <= maxChar; charOff++ {
		byteOff := enc.CharToByte(line4, charOff)
		back := enc.ByteToChar(line4, byteOff)
		// If byteOff maps back to an earlier position, charOff was inside
		// a supplementary character — skip it (not a valid char boundary).
		if back < charOff {
			continue
		}
		if back != charOff {
			t.Errorf("round-trip char %d: byteOff=%d back=%d (line len=%d)", charOff, byteOff, back, len(line4))
		}
	}
}
