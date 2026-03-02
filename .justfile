build-lsp:
    go build -o obsidian_ls ./cmd/obsidian_ls
    cp -f obsidian_ls $GOBIN/
