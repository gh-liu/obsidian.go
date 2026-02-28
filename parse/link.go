package parse

// LinkKind distinguishes wiki links from markdown links.
type LinkKind int

const (
	LinkWiki LinkKind = iota
	LinkMarkdown
)

// Link represents a link to another note or resource.
// Target: at parse, stub Doc with ID=id; at index, resolved Doc. Nil for same-note [[#heading]].
// Anchor: *Heading for #heading (Text=id), *Block for #^block-id (ID=id).
type Link struct {
	Kind   LinkKind
	Target *Doc     // Path holds id at parse; resolved at index; nil = same-note
	Anchor *Heading // for #heading, Text holds id
	Block  *Block   // for #^block-id, ID holds id
	Alias  string   // for wiki [[note|alias]]
	Range  Range
}
