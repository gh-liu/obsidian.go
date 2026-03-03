# obsidian.go

LSP server for Obsidian vaults. Provides go-to-definition, completion, outline, and more for Markdown.

## Features

| Capability           | Description                                                           |
|----------------------|-----------------------------------------------------------------------|
| **Go to Definition** | Jump from `[[file]]`, `[[#heading]]`, `[[path#heading]]` to target    |
| **Find References**  | Find all references to a note                                         |
| **Completion**       | Trigger on `[` or `#` for wiki link / heading completion              |
| **Document Symbol**  | Document outline (heading tree)                                       |
| **Format**           | Format frontmatter (id, title, createdAt, updatedAt)                  |
| **Execute Command**  | `obsidian.new`, `obsidian.newFromTemplate`, `obsidian.insertTemplate` |

## Installation

```bash
go install github.com/gh-liu/obsidian.go/cmd/obsidian_ls@latest
```

Or build locally:

```bash
go build -o obsidian_ls ./cmd/obsidian_ls
# Ensure the binary is in PATH, or point your editor to its path
```

## Editor Setup

Configure `obsidian_ls` as the LSP server for Markdown in your vault. 

## nvim

```lua
vim.lsp.config("obsidian_ls", {
	cmd = { "obsidian_ls" },
	filetypes = { "markdown" },
	root_markers = { ".templates" },
	settings = {
		["obsidian"] = {
			ignores = {
				"^.templates/",
				"^blog/",
			},
		},
	},
})
```

## Configuration

| Setting                 | Description                                 |
|-------------------------|---------------------------------------------|
| `obsidian.ignores`      | Array of regex patterns for paths to ignore |
| `obsidian.templatePath` | Template directory, default `.templates`    |
