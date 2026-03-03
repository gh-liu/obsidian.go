package parse

import "strings"

// WikiLinkCursorContext describes completion context inside a wiki link [[...]]
// at a given cursor byte offset.
type WikiLinkCursorContext struct {
	StartByte     int
	Prefix        string
	CompleteFiles bool
	CompleteBlock bool
	TargetPath    string
}

// ParseWikiLinkCursorContext parses the wiki-link context at byteOff in line.
// It returns nil when cursor is not inside an unclosed wiki link.
func ParseWikiLinkCursorContext(line string, byteOff int) *WikiLinkCursorContext {
	if byteOff > len(line) {
		byteOff = len(line)
	}
	beforeCursor := line[:byteOff]

	open := -1
	// Linear scan to locate the last unclosed `[[` before cursor.
	for i := 0; i < len(beforeCursor)-1; i++ {
		if beforeCursor[i] == '[' && beforeCursor[i+1] == '[' {
			open = i + 2
			i++
			continue
		}
		if beforeCursor[i] == ']' && beforeCursor[i+1] == ']' {
			open = -1
			i++
		}
	}
	if open < 0 {
		return nil
	}

	inner := beforeCursor[open:byteOff]
	if strings.Contains(inner, "|") {
		return nil
	}
	if after, ok := strings.CutPrefix(inner, "#"); ok {
		if blockPrefix, ok := strings.CutPrefix(after, "^"); ok {
			return &WikiLinkCursorContext{
				StartByte:     open + 2,
				Prefix:        blockPrefix,
				CompleteFiles: false,
				CompleteBlock: true,
				TargetPath:    "",
			}
		}
		prefix := after
		lastHash := strings.LastIndex(prefix, "#")
		if lastHash >= 0 {
			prefix = prefix[lastHash+1:]
			headingStartByte := open + 2 + lastHash
			return &WikiLinkCursorContext{
				StartByte:     headingStartByte,
				Prefix:        prefix,
				CompleteFiles: false,
				CompleteBlock: false,
				TargetPath:    "",
			}
		}
		return &WikiLinkCursorContext{
			StartByte:     open + 1,
			Prefix:        prefix,
			CompleteFiles: false,
			CompleteBlock: false,
			TargetPath:    "",
		}
	}
	if idx := strings.LastIndex(inner, "#"); idx >= 0 {
		targetPath := strings.TrimSpace(inner[:idx])
		prefix := inner[idx+1:]
		startByte := open + idx + 1
		completeBlock := false
		if blockPrefix, ok := strings.CutPrefix(prefix, "^"); ok {
			completeBlock = true
			prefix = blockPrefix
			startByte++
		}
		return &WikiLinkCursorContext{
			StartByte:     startByte,
			Prefix:        prefix,
			CompleteFiles: false,
			CompleteBlock: completeBlock,
			TargetPath:    targetPath,
		}
	}
	return &WikiLinkCursorContext{
		StartByte:     open,
		Prefix:        inner,
		CompleteFiles: true,
		CompleteBlock: false,
		TargetPath:    "",
	}
}
