package parse

import "strings"

// parseWikiLinkParts splits a wiki link inner content into (target, anchor/blockRef, alias).
// Input is the content between [[ and ]], e.g. "note#heading|alias" or "note|alias".
// Returns: target, anchorOrBlockRef, alias.
//   - anchorOrBlockRef contains "#" + heading or "#^" + block-id (or empty).
//     The "#" prefix allows downstream to distinguish heading from block via ^ prefix.
func parseWikiLinkParts(inner string) (target, anchorBlock, alias string) {
	// Split alias: [[target#anchor|alias]]
	if idx := strings.LastIndex(inner, "|"); idx >= 0 {
		alias = strings.TrimSpace(inner[idx+1:])
		inner = inner[:idx]
	}
	// Split anchor/block: [[target#anchor]] or [[#anchor]] or [[target#^block-id]]
	if idx := strings.Index(inner, "#"); idx >= 0 {
		target = strings.TrimSpace(inner[:idx])
		anchorBlock = inner[idx:] // includes "#" prefix
		return
	}
	target = strings.TrimSpace(inner)
	return
}
