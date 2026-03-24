package parse

import "strings"

func splitWikiAlias(inner string) (targetPart, alias string, hasAlias bool, sepByte int) {
	for i := 0; i < len(inner); i++ {
		if inner[i] != '|' {
			continue
		}
		if i > 0 && inner[i-1] == '\\' {
			return inner[:i-1], inner[i+1:], true, i
		}
		return inner[:i], inner[i+1:], true, i
	}
	return inner, "", false, -1
}

func unescapeWikiPipes(s string) string {
	return strings.ReplaceAll(s, `\|`, `|`)
}

func parseWikiLinkParts(inner string) (targetID, anchorID, alias string) {
	targetPart, alias, hasAlias, _ := splitWikiAlias(inner)
	if hasAlias {
		alias = unescapeWikiPipes(alias)
	}
	targetPart = unescapeWikiPipes(targetPart)
	targetID = strings.TrimSpace(targetPart)
	if idx := strings.Index(targetPart, "#"); idx >= 0 {
		targetID = strings.TrimSpace(targetPart[:idx])
		anchorID = strings.TrimSpace(unescapeWikiPipes(targetPart[idx+1:]))
	}
	return targetID, anchorID, alias
}
