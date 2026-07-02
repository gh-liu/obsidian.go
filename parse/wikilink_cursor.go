package parse

import "strings"

// WikiLinkCursorContext describes the completion context when the cursor is
// inside or immediately after a wiki link [[...]].
type WikiLinkCursorContext struct {
	StartByte     int    // byte offset of '[' that starts the wiki link
	EndByte       int    // byte offset after ']' that ends the wiki link (or 0 if incomplete)
	Prefix        string // text from the start of the current segment for filtering
	CompleteFiles bool
	CompleteBlock bool
	CompleteAlias bool
	TargetPath    string // the resolved file path (from Target) for heading/block completion
	TargetAnchor  string // the heading part for cross-note heading completion
}

// ParseWikiLinkCursorContext analyzes the cursor position within a line
// and returns the completion context for [[...]] syntax.
// byteOff is the cursor byte offset (UTF-8) within the line.
func ParseWikiLinkCursorContext(line string, byteOff int) *WikiLinkCursorContext {
	// Find the nearest [[ ]] pair that contains or precedes the cursor.
	linkStart := -1
	linkEnd := -1
	isComplete := false

	// Search for the [[ that contains or precedes the cursor.
	for i := 0; i < len(line)-1; i++ {
		if line[i] != '[' || line[i+1] != '[' {
			continue
		}
		if i > 0 && line[i-1] == '\\' {
			continue
		}
		// Find closing ]]
		closeEnd := -1
		for j := i + 2; j < len(line)-1; j++ {
			if line[j] == '\\' && j+1 < len(line) {
				j++
				continue
			}
			if line[j] == ']' && line[j+1] == ']' {
				closeEnd = j + 2 // past the second ']'
				break
			}
		}
		if closeEnd > 0 && byteOff >= i && byteOff <= closeEnd {
			// Cursor is inside a complete [[...]].
			linkStart = i
			linkEnd = closeEnd - 2 // point at the start of ]]
			isComplete = true
			break
		}
		if closeEnd < 0 {
			// Incomplete link (no closing ]]). Check if cursor is after [[
			if byteOff >= i+2 && (byteOff <= len(line)) {
				linkStart = i
				linkEnd = byteOff
				isComplete = false
				break
			}
		}
	}

	if linkStart < 0 {
		return nil
	}
	// For complete links where cursor is at or after the closing ]], don't offer completion.
	if isComplete && byteOff >= linkEnd+2 {
		return nil
	}

	inner := ""
	if linkEnd > linkStart+2 {
		endIdx := linkEnd
		if isComplete {
			// linkEnd points at the start of ]] (excluded from inner).
			// For complete links, inner = content between [[ and ]].
		} else {
			// Incomplete link: linkEnd = byteOff, inner includes everything after [[
		}
		if endIdx > len(line) {
			endIdx = len(line)
		}
		inner = line[linkStart+2 : endIdx]
	} else {
		inner = ""
	}

	target, anchorBlock, _ := parseWikiLinkParts(inner)

	ctx := &WikiLinkCursorContext{
		StartByte: linkStart,
		EndByte:   linkEnd,
		Prefix:    inner,
	}

	// Determine what to complete based on the cursor's position in the link.
	afterHash := strings.LastIndex(inner, "#")
	afterPipe := strings.LastIndex(inner, "|")
	afterCaret := strings.LastIndex(inner, "^")

	var justAfterPipe bool
	if afterPipe >= 0 {
		// Cursor is in or after the alias section.
		aliasStart := afterPipe + 1
		if byteOff-linkStart-2 >= aliasStart {
			ctx.CompleteAlias = true
			ctx.TargetPath = target
			ctx.Prefix = strings.TrimPrefix(inner[aliasStart:], " ")
			justAfterPipe = true
			return ctx
		}
	}

	if afterHash >= 0 {
		hashIdx := afterHash + 1 // position after '#'
		if justAfterPipe && byteOff-linkStart-2 < hashIdx {
			// Cursor is between target and # in alias section - shouldn't happen but handle.
			return ctx
		}
		if afterCaret >= 0 && byteOff-linkStart-2 >= afterCaret {
			// Completing block ID: [[target#^...]]
			ctx.CompleteBlock = true
			ctx.TargetPath = target
			ctx.Prefix = strings.TrimPrefix(inner[afterCaret+1:], " ")
			return ctx
		}
		// Completing heading: [[#...]] or [[target#...]]
		ctx.TargetPath = target
		ctx.TargetAnchor = strings.TrimPrefix(anchorBlock, "#")
		prefix := inner[hashIdx:]
		ctx.Prefix = prefix
		return ctx
	}

	// Default: completing file/target.
	ctx.CompleteFiles = true
	ctx.Prefix = target
	return ctx
}
