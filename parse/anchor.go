package parse

// Heading represents a markdown heading (h1-h6).
type Heading struct {
	Level int
	Text  string
	Range Range
}

// Block represents a block reference (#^block-id).
type Block struct {
	ID    string
	Range Range
}
