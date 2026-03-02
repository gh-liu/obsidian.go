package parse

import "testing"

func TestParseWikiLinkCursorContext(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		byteOff int
		wantNil bool
		want    *WikiLinkCursorContext
	}{
		{name: "not in link", line: "plain text", byteOff: 5, wantNil: true},
		{name: "closed link", line: "[[path]]", byteOff: 8, wantNil: true},
		{name: "cursor before [[", line: "x[[path", byteOff: 1, wantNil: true},
		{name: "file empty prefix", line: "[[", byteOff: 2, wantNil: false, want: &WikiLinkCursorContext{StartByte: 2, Prefix: "", CompleteFiles: true, TargetPath: ""}},
		{name: "file with path", line: "See [[path", byteOff: 11, wantNil: false, want: &WikiLinkCursorContext{StartByte: 6, Prefix: "path", CompleteFiles: true, TargetPath: ""}},
		{name: "file partial path", line: "[[sub/note", byteOff: 10, wantNil: false, want: &WikiLinkCursorContext{StartByte: 2, Prefix: "sub/note", CompleteFiles: true, TargetPath: ""}},
		{name: "same-note heading empty", line: "[[#", byteOff: 3, wantNil: false, want: &WikiLinkCursorContext{StartByte: 3, Prefix: "", CompleteFiles: false, TargetPath: ""}},
		{name: "same-note heading", line: "[[#Section", byteOff: 10, wantNil: false, want: &WikiLinkCursorContext{StartByte: 3, Prefix: "Section", CompleteFiles: false, TargetPath: ""}},
		{name: "nested heading", line: "[[#H1#H2", byteOff: 9, wantNil: false, want: &WikiLinkCursorContext{StartByte: 6, Prefix: "H2", CompleteFiles: false, TargetPath: ""}},
		{name: "cross-file heading", line: "[[file#heading", byteOff: 15, wantNil: false, want: &WikiLinkCursorContext{StartByte: 7, Prefix: "heading", CompleteFiles: false, TargetPath: "file"}},
		{name: "cross-file heading empty", line: "[[path#", byteOff: 7, wantNil: false, want: &WikiLinkCursorContext{StartByte: 7, Prefix: "", CompleteFiles: false, TargetPath: "path"}},
		{name: "cross-file with spaces", line: "[[ path # sub", byteOff: 14, wantNil: false, want: &WikiLinkCursorContext{StartByte: 9, Prefix: " sub", CompleteFiles: false, TargetPath: "path"}},
		{name: "second link on line", line: "[[a]] and [[b", byteOff: 16, wantNil: false, want: &WikiLinkCursorContext{StartByte: 12, Prefix: "b", CompleteFiles: true, TargetPath: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseWikiLinkCursorContext(tt.line, tt.byteOff)
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseWikiLinkCursorContext() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseWikiLinkCursorContext() = nil, want %+v", tt.want)
			}
			if got.StartByte != tt.want.StartByte || got.Prefix != tt.want.Prefix ||
				got.CompleteFiles != tt.want.CompleteFiles || got.TargetPath != tt.want.TargetPath {
				t.Errorf("ParseWikiLinkCursorContext() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
