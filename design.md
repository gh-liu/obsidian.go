# obsidian.go 设计概览

`obsidian.go` 是一个面向 Obsidian vault 的 Markdown LSP Server。它通过标准输入/输出运行 JSON-RPC LSP 协议，为编辑器提供 Obsidian 风格的链接导航、补全、引用、诊断、模板和格式化能力。

## 总体结构

- `cmd/obsidian_ls`: 可执行入口，初始化日志并启动 LSP server。
- `internal/lsp`: LSP 协议处理层，负责初始化、文档同步、索引更新、能力分发和具体 LSP feature。
- `internal/lsp/index`: vault 索引，维护 Markdown 文档、frontmatter id、文件 basename、打开文件未保存内容等映射。
- `internal/lsp/completion`: wiki link、heading、block、alias 补全。
- `internal/lsp/format`: frontmatter 默认字段格式化。
- `internal/lsp/template`: 模板加载、变量替换和模板列表。
- `parse`: Markdown/Obsidian 语法解析，提取 frontmatter、heading、block id、wiki link、markdown link 等结构。

## 已提供能力

### LSP 基础能力

- 作为 `obsidian_ls` 二进制运行，通过 stdin/stdout 提供 LSP 服务。
- 支持 `textDocument/didOpen`、`didChange`、`didClose`，维护打开文件的未保存内容。
- 初始化后扫描 workspace 下的 Markdown 文件并建立 vault 索引。
- 动态注册 `**/*.md` 文件监听，基于客户端的 `workspace/didChangeWatchedFiles` 增量更新索引。
- 支持 `utf-8` 和 `utf-16` position encoding，默认 `utf-16`。
- 从客户端读取 `obsidian` workspace configuration。

### Vault 索引与解析

- 并发扫描并解析 vault 中的 `.md` 文件。
- 支持按以下方式解析和索引文档：
  - frontmatter: `id`、`title`、`aliases`、`tags`、`createdAt`、`updatedAt`。
  - Markdown heading。
  - Obsidian block id（`^block-id`）。
  - Obsidian wiki link（`[[...]]`）。
  - Markdown link（`[text](target)`）。
- 支持通过 frontmatter `id`、相对路径、带/不带 `.md` 后缀路径、basename 解析笔记目标。
- 对打开但未保存的文件内容进行单独覆盖索引，避免补全和诊断只依赖磁盘状态。

### 跳转定义

支持 `textDocument/definition`，可从 wiki link 跳转到目标：

- `[[file]]` / `[[path/to/file]]`：跳转到笔记文件。
- `[[id]]`：通过 frontmatter `id` 跳转到笔记文件。
- `[[file#heading]]`：跳转到目标文件中的 heading。
- `[[#heading]]`：跳转到当前文件中的 heading。
- `[[file#^block-id]]` / `[[#^block-id]]`：跳转到目标 block id。

### 查找引用

支持 `textDocument/references`：

- 查询当前文件的反向链接。
- 当前仅统计指向当前文件的 Obsidian wiki link（`[[file]]`、`[[id]]`、`[[file#heading]]` 等）。
- heading 级引用先保留设计，不在当前实现中启用。

### 补全

支持 `textDocument/completion`，触发字符为 `[`, `#`, `|`：

- wiki link 文件补全：`[[...]]`。
- 当前文件 heading 补全：`[[#...]]`。
- 跨文件 heading 补全：`[[file#...]]`。
- block id 补全：`[[file#^...]]` 或 `[[#^...]]`。
- alias 补全：`[[file|...]]`。
- 文件补全结果最多返回 100 条，并根据上下文决定是否标记 incomplete。

### 文档结构与全局符号

- 支持 `textDocument/documentSymbol`，返回当前 Markdown 文档的 heading tree，用作大纲。
- 支持 `workspace/symbol`：
  - 搜索笔记文件名。
  - 搜索 heading。
  - 支持 tag 过滤查询，例如 `#tag` 或 `#tag1,tag2 keyword`。
  - 最多返回 200 条结果。

### 诊断与快速修复

- 对打开文件发布未解析 wiki link 诊断：`broken-link`。
- 文件索引变化后会重新诊断打开文件。
- 支持 `textDocument/codeAction` 快速修复：对 broken link 提供 `Create note '<target>'`，创建对应笔记。

### CodeLens 与引用展示

- 支持 `textDocument/codeLens`：
  - 在 frontmatter `id` 上显示文件级引用数量。
  - heading 引用数量先保留设计，不在当前实现中启用。
- CodeLens 命令使用 `obsidian.showReferences`，打开第一个引用位置；多引用时显示提示消息。

### Inlay Hint

支持自定义注册的 `textDocument/inlayHint`：

- 为 wiki link 显示 `-> ...` 提示。
- 优先展示目标文档 frontmatter `title`。
- 没有 title 时回退到目标路径。
- 对 heading / block reference 追加 `#heading` 或 `#^block-id` 信息。
- 如果 alias 与展示文本等价，则只显示 `->`，避免重复。

### 格式化

支持 `textDocument/formatting`：

- 为 Markdown 文件维护 frontmatter 默认字段：`id`、`title`、`createdAt`、`updatedAt`。
- 没有 frontmatter 时会创建 frontmatter。
- 已有 frontmatter 时补齐缺失字段。
- `updatedAt` 会在格式化时刷新。

### 模板与命令

支持 `workspace/executeCommand`：

- `obsidian.new`: 使用默认模板创建新笔记。
- `obsidian.newFromTemplate`: 使用指定模板创建新笔记。
- `obsidian.insertTemplate`: 在当前光标位置插入指定模板内容。
- `obsidian.listTemplates`: 返回模板名列表，供模板选择器补全。
- `obsidian.createNote`: 为 broken link 创建笔记。
- `obsidian.showReferences`: 打开引用位置。

模板能力：

- 默认模板目录为 `.templates`，可配置。
- 模板文件为 `<name>.md`。
- 内置默认模板 `default`，当 `.templates/default.md` 不存在时使用。
- 支持 Obsidian Templates 风格变量：`{{title}}`、`{{date}}`、`{{time}}`、`{{id}}`。
- 模板创建的笔记会确保 frontmatter 中存在 `id`。

## 配置

当前支持的 `obsidian` 配置项：

| 配置项 | 说明 |
| --- | --- |
| `obsidian.ignores` | 正则数组，匹配后跳过索引、格式化和创建等相关操作。 |
| `obsidian.templatePath` | 模板目录，相对 vault root，默认 `.templates`。 |

## 当前边界

- 索引范围主要是 workspace/vault 下的 `.md` 文件。
- 文件变化依赖 LSP 客户端发送 `workspace/didChangeWatchedFiles`，服务端自身不直接监听文件系统。
- Markdown 解析是轻量级行解析，聚焦 frontmatter、heading、block id 和链接，不是完整 Markdown AST。
- `README.md` 中列出的命令是主要公开命令；代码中还包含内部用于 quick fix 和引用展示的 `obsidian.createNote`、`obsidian.showReferences`。

---

## 核心能力分析

### 一句话本质

**将 Obsidian vault（文件夹 + Markdown 文件）转化为可通过 LSP 协议导航、查询、编辑的双向知识图谱。**

核心路径是 4 步 pipeline：**Parse → Index → Consistency → LSP**。缺任何一步，功能都不完整。

---

## 1. 解析（Parse）

**目标**：从 Markdown 文本中分离 frontmatter 与 body content，并从 body 中提取 heading、wiki link、block id、markdown link 等语义结构。

### 输出数据模型：`parse.Doc`

```go
Doc {
    Path                           // 相对路径
    ID, Title, Aliases, Tags       // frontmatter 字段
    CreatedAt, UpdatedAt           // frontmatter 时间
    Headings []*Heading            // 行级 # heading
    Blocks   []*Block              // 行级 ^block-id
    Links    []*Link               // 行级 [[...]] 和 [text](url)
}
```

解析器是纯函数：`(rawContent, path) → *Doc`。每个文件独立解析，无跨文件依赖。

### 解析覆盖的语法

| 元素 | 示例 | 产出 |
|------|------|------|
| YAML frontmatter | `---\nid: foo\n---` | `Doc.ID`, `Doc.Title`, `Doc.Aliases`, `Doc.Tags` 等 |
| Heading | `# Title`, `## Sub` | `Heading{Level, Text, Range}` |
| Wiki link | `[[target]]` | `Link{Kind:Wiki, Target}` |
| Wiki link + heading | `[[file#heading]]` | `Link{Target:"file", Anchor:"heading"}` |
| Wiki link + block | `[[#^block-id]]` | `Link{Target:"", BlockRef:"block-id"}` |
| Wiki link + alias | `[[note|display]]` | `Link{Target:"note", Alias:"display"}` |
| Markdown link | `[text](url)` | `Link{Kind:Markdown, Target:"url"}` |
| Block ID | `text ^block-id` | `Block{ID, Range}` |

### Link 结构设计

```go
type Link struct {
    Target   string  // 目标路径/ID；空 = 当前文件内链接
    Anchor   string  // heading 文本（无 # 前缀）
    BlockRef string  // block ID（无 ^ 前缀）
    Alias    string  // 显示别名
    Range    Range   // 在原文中的 0-based [start, end)
}
```

Anchor 与 BlockRef 互斥：`^` 前缀区分 block ref 和 heading anchor。

**示例映射**：
- `[[file#heading]]` → `Target="file", Anchor="heading"`
- `[[#^block-id]]` → `Target="", BlockRef="block-id"`
- `[[note|alias]]` → `Target="note", Alias="alias"`

### 解析策略

- **行级正则匹配**，不构建完整 Markdown AST。LSP 场景优先低延迟而非完备性。
- **Range 记录**：每个结构元素记录 `{Line, Character}` range，内部使用 **UTF-8 byte offset** 作为 Character 单位，减少多字节字符下的转换开销。LSP 协议边界通过 `position.Encoder` 做 UTF-8 ↔ UTF-16 转换。
- frontmatter 解析依赖 `gopkg.in/yaml.v3`，支持字符串或字符串数组的 `aliases` / `tags`，以及多种时间格式的 `createdAt` / `updatedAt`。

### Heading Anchor 规范化（基础设施）

Obsidian 将 heading 文本转为 URL-safe anchor 用于 `[[note#anchor]]` 跳转。规则：
1. 小写化，空格/制表符替换为 `-`。
2. 保留字母、数字、下划线；删除其他字符。
3. 重复 heading 添加数字后缀：`my-title`、`my-title-1`、`my-title-2`。

`anchors.go` 按文档顺序生成 anchor 列表，与 Obsidian 行为一致。这是跳转和引用的基础——缺少它，`[[note#heading]]` 无法定位。

---

## 2. 索引（Index）

**目标**：以 `id` 为主键建立 `id → Doc` 的映射，同时支持按路径、basename 等多维度查询。

### 索引结构

```go
Index {
    byPath         map[string]*Doc        // relPath → Doc（主存储）
    byID           map[string]string      // id → relPath（id 唯一，直接定位）
    byBasename     map[string][]string    // basename → []relPath（回退查询）
    contentByPath  map[string]string      // 打开文件覆盖层（见第 3 步）
}
```

id 是唯一的，一个 id 只对应一个文件，不存在冲突。`byID` 是二次跳转 key：先拿到 path，再通过 `byPath[path]` 拿到 Doc。这样当文件 id 变更时只需更新 `byID` 中的映射，不影响 `byPath` 中已有的 Doc。

### 需要的索引维度

| 维度 | 用途 | 是否必须 |
|------|------|----------|
| `id → Doc` | wiki link 目标解析（`[[id]]`）、CodeLens 引用计数 | **必须** |
| `relPath → Doc` | wiki link 路径解析（`[[path/file]]`）、文件内容查询 | **必须** |
| `basename → []relPath` | 不完整路径回退（用户只写 `[[note]]` 不写路径前缀时） | 强烈建议 |
| `id → title` | 跳转结果的 display name、InlayHint 展示 | 可选（从 `Doc.Title` 间接获取） |

如果进一步扩展，可考虑：
- `byTag map[string][]string`：tag → 使用该 tag 的文件列表，支撑 tag 补全和 tag 跳转。
- `byHeadingAnchor map[string][]*HeadingRef`：跨文件的 heading anchor 索引，加速 `[[note#heading]]` 的 heading 匹配。目前是 O(n) 扫描目标文件的 Doc.Headings，对于大 vault 仍然可接受。

### 查找链（`ResolveLinkTargetToPath`）

给定 wiki link 的 target 字符串，按优先级解析：

```
1. byID[target]          → 找到 target 是某文件的 frontmatter id
2. byPath[target]        → 找到 target 本身是文件路径
3. byPath[target+".md"]  → 补齐后缀重试（兼容用户省略 .md）
4. byBasename[base] → 选最短路径     → 仅凭文件名匹配（多个候选时取最短）
5. 遍历 contentByPath 中的 Doc 做 id 匹配 → 新建未保存文件回退
```

匹配到则返回目标文件的 `relPath`，否则返回空字符串（触发 broken-link 诊断）。

此链完整覆盖 Obsidian 链接解析行为：ID 优先 > 精确路径 > 后缀补齐 > basename 回退。

### 全量索引

`IndexAll()` 在 `Initialized` 后异步执行：`filepath.Walk` 收集所有 `.md` 文件 → `errgroup` 并发解析（并发度 `runtime.NumCPU() * 2`）→ 写入 `byPath` / `byID` / `byBasename`。

---

## 3. 数据一致性（Consistency）

**问题**：文件随时可能被创建、修改、删除，或被编辑器打开未保存。索引里的 Doc 必须始终反映"当前最新状态"，否则补全给出过期结果、跳转指向错误位置、诊断发布误报。

这是整个系统中**情况最多、最容易出 bug 的环节**。

### 状态源优先级

```
编辑器未保存内容  >  磁盘文件  >  不存在（文件已删除/尚未创建）
```

### 事件 → 动作矩阵

| 事件 | 触发条件 | 处理 |
|------|----------|------|
| **文件打开** | `textDocument/didOpen` | `SetContent(path, content)`：写入覆盖层 → debounce 100ms 后异步重解析 → 更新 `byPath/byID/byBasename` |
| **编辑（未保存）** | `textDocument/didChange` | 同上。每次编辑更新覆盖层，debounce 合并快速连续输入 |
| **文件关闭** | `textDocument/didClose` | `ClearContent(path)`：删除覆盖层 → 从磁盘回读 → 重解析 → 更新索引。如果磁盘文件已不存在，从索引中移除 |
| **磁盘新建** | `didChangeWatchedFiles` Created | `Add(path, diskContent)`：解析并插入索引 |
| **磁盘修改** | `didChangeWatchedFiles` Changed | **若 `HasOpenContent(path)` → 跳过**（编辑器中未保存内容优先）。否则 `Update(path, diskContent)` |
| **磁盘删除** | `didChangeWatchedFiles` Deleted | `Remove(path)`：从 `byPath/byID/byBasename` 中清除 |
| **id 变更** | 用户在 frontmatter 修改 id + debounce 重解析触发 | `removeDocLocked` 清除旧 id → `addDocLocked` 写入新 id。通过先删后加保证 `byID` 不残留 |
| **文件重命名** | 客户端通常发 `didClose`(旧) + `didChangeWatchedFiles` Created(新) + Deleted(旧) | 组合处理：旧路径清除，新路径插入 |

### 覆盖层与 Debounce（关键设计）

`contentByPath` 是"打开文件覆盖层"：

```
GetContent(path):
  1. 如果 contentByPath[path] 存在 → 返回覆盖层内容
  2. 否则 → 从磁盘读取
```

`SetContent()` 写入覆盖层后不立即重解析，而是通过 100ms debounce timer 延迟触发。原因：
- 用户快速打字时每次 keystroke 就是一次 `didChange`，如果不 debounce 会频繁重解析。
- 重解析需要写锁（`byPath/byID/byBasename`），频繁加锁会阻塞补全和跳转查询的读锁。
- 100ms 在"打字流畅性"和"补全及时性"之间取得平衡。

```go
SetContent(path, content):
  contentByPath[path] = content     // 立即写入覆盖层（读路径可立即看到最新内容）
  scheduleReparse(path)             // 延迟重解析（更新 byPath/byID 中的 Doc）
```

### 索引变更的级联影响

每次索引变更后，不仅要更新 `byPath/byID/byBasename`，还需要：
1. **重新诊断所有打开文件**（因为 A 文件的 link 引用了 B 文件的 id，B 变更可能导致 A 的 broken-link 状态改变）。
2. 诊断刷新也有 120ms debounce，避免连续多次索引变更触发多次诊断。

---

## 4. LSP 接口能力

**目标**：基于索引，通过标准 LSP 协议向编辑器提供 wiki link 补全、跳转定义、引用查找、诊断等功能。

### 新增 Feature 的一般模式

```
handler.go: 注册 LSP method → 提取 relPath、encoding → 调用 ResolveXxx()
feature.go:  ResolveXxx() 实现语义逻辑：用 index 查询 → 返回 protocol.* 类型结果
```

Handler 不做语义逻辑，feature 文件不处理协议适配。`index/` 只提供数据访问，不包含任何 LSP 概念。

### 已实现的 LSP Feature

| Feature | LSP Method | 核心逻辑 |
|---------|-----------|----------|
| **Go to Definition** | `textDocument/definition` | 取光标位置的 Link → `ResolveLinkTargetToPath()` 定位目标文件 → 若 Link 有 Anchor/BlockRef 则定位到 heading/block 行 |
| **Find References** | `textDocument/references` | 扫描全 vault 的 Obsidian wiki link，返回 target 指向当前文件的反向链接；heading 级引用保留设计但暂不启用 |
| **Completion** | `textDocument/completion` | 解析光标在 `[[...]]` 内部的**状态**（在填文件名/heading/block id/alias？）→ 分派到对应补全函数 |
| **Diagnostics** | `textDocument/publishDiagnostics` | 遍历当前文件所有 Link → `ResolveLinkTargetToPath()` 为空则发布 `broken-link` 警告 |
| **Code Action** | `textDocument/codeAction` | 对 broken-link 诊断提供 `Create note` 快速修复 → 调用 `obsidian.createNote` 命令创建目标文件 |
| **CodeLens** | `textDocument/codeLens` | 在 frontmatter `id` 上显示文件级引用计数；heading 引用计数保留设计但暂不启用 |
| **Inlay Hint** | `textDocument/inlayHint`（自定义注册） | 为 wiki link 显示目标 title 或路径 |
| **Document Symbol** | `textDocument/documentSymbol` | 返回当前文件 heading tree |
| **Workspace Symbol** | `workspace/symbol` | 搜索文件名、heading、tag |
| **Formatting** | `textDocument/formatting` | 补齐 frontmatter 默认字段（id/title/createdAt/updatedAt） |

### 补全系统：状态驱动的上下文识别

补全是整个 LSP 接口中最复杂的 feature。不能简单匹配字符串——必须先识别光标在 wiki link 内部的**确切位置**（状态），再分派到不同补全策略。

由 `parse.ParseWikiLinkCursorContext(line, byteOff)` 实现光标上下文分析：

| 上下文 | 光标位置 | 补全内容 |
|--------|----------|----------|
| `completeFiles` | `[[...` | vault 中所有 .md 文件路径 |
| `completeBlocks` | `[[file#^...` | 目标文件中的 block id 列表 |
| `completeAlias` | `[[file|...` | 目标文件的 alias 列表 |
| heading 补全 | `[[#...` 或 `[[file#...` | 当前/目标文件的 heading |

文件补全上限 100 条，超出部分标记 `IsIncomplete` 让客户端二次筛选。

### 位置编码适配

内部所有位置使用 **UTF-8 byte offset**。但 LSP 客户端可能使用 UTF-16 编码位置。`position.Encoder` 在协议边界做双向转换：
- `ByteToChar(line, byteOff) → charOff`：将内部 offset 转为客户端期望的编码。
- `CharToByte(line, charOff) → byteOff`：将客户端发来的位置转为内部 offset。

### 增量索引与动态注册

- `Initialized` 后异步执行 `IndexAll()`。
- 通过 `client/registerCapability` 动态注册 `**/*.md` 文件监听。
- 之后所有索引更新由 `workspace/didChangeWatchedFiles` 驱动（增/改/删）。

### 架构决策与权衡

| 决策 | 理由 |
|------|------|
| 轻量行解析而非完整 AST | LSP 需要低延迟；完整 Markdown AST 的成本远高于收益。 |
| 内存索引而非数据库 | Vault 通常小于 10K 文件，内存索引足够；无外部依赖。 |
| debounce 100ms 重解析 | 平衡打字响应性和解析准确性。 |
| 打开文件覆盖层 | 编辑器是 truth source，磁盘仅作回退。 |
| 客户端文件监听 | 避免服务端 inotify/fsnotify 跨平台兼容性问题。 |
| `go.lsp.dev` 协议库 | 标准化 LSP 3.17 实现，减小协议适配工作量。 |

### 未来扩展方向

#### Tier 1 — 高价值、低复杂度

| 能力 | 说明 | 实现思路 |
|------|------|----------|
| **Hover** | 悬停 wiki link 显示目标笔记预览/backlinks 数量 | 复用 InlayHint 的目标解析逻辑 + 读取目标文件前几行；注册 `textDocument/hover` |
| **Rename (笔记重命名)** | 重命名 .md 文件并更新所有 `[[...]]` 引用 | `textDocument/rename` + `workspace/willRenameFiles`：扫描全 vault link → 找出引用目标 → 生成 TextEdit 替换；配合文件系统操作 |
| **Semantic Tokens** | 为 wiki link、block id、tag 提供语法高亮 | 遍历 Doc 中 Links/Blocks/Headings，按 LSP semantic token delta 协议推送 token 数组 |
| **Folding Range** | Heading-based 折叠区域 | 基于 Doc.Headings 生成 folding range（从 heading 行到下一个同级/上级 heading 之前） |

#### Tier 2 — 中复杂度

| 能力 | 说明 | 实现思路 |
|------|------|----------|
| **Embed 支持** | `![[note]]` 嵌入语法（跳转、补全、诊断、inlay hint） | 在 `parseLinks()` 中扩展识别 `![[...]]` 前缀；Link 结构新增 `Embed bool`；所有 feature 复用现有 link 处理，仅 InlayHint 可显示嵌入预览 |
| **Tag 一等公民** | Tag 补全、tag 跳转、tag 引用统计 | `byTag map[string][]string` 索引；`workspace/symbol` 已有 tag 过滤，扩展为补全和跳转 |
| **Daily Note** | `obsidian.openToday` 创建/打开当日笔记 | 配置 daily note 路径格式（如 `daily/YYYY-MM-DD.md`），命令创建笔记并使用 daily note 模板 |
| **Callout / Admonition** | Obsidian callout 块的语义高亮 | Semantic tokens 或诊断提示；识别 `> [!note]` / `> [!warning]` 等块语法 |

#### Tier 3 — 高复杂度

| 能力 | 说明 | 实现思路 |
|------|------|----------|
| **Graph View (工作区符号增强)** | 在 `workspace/symbol` 中支持图谱局部查询 | 给定当前文件，返回直接前向链接和反链作为结构化结果；可复用现有引用查找 |
| **Properties (Frontmatter v2)** | 支持 Obsidian 新 properties 格式（`key:: value` inline） | 扩展 `parseFrontmatter` 识别 properties 语法；与 YAML frontmatter 并行处理 |
| **跨 Vault 链接** | 支持跨 vault 的 `[[vault:note]]` 链接 | 需要在 Initialize 时识别多 workspace folder；建立 per-vault 索引 |
| **Dataview 支持** | 识别 Dataview 查询块中的隐式依赖 | 解析 dataview 代码块中的 `FROM`、`WHERE file =` 等模式，提取引用的页面/标签作为隐式链接 |

### 扩展实现的一般模式

任何新 LSP feature 的实现可以遵循以下模式（project 已建立好的 convention）：

```
1. 在 handler.go 中注册 LSP method → Handler 方法
2. Handler 方法做：
   a. 检查 index != nil
   b. uriToRelPath 提取文件相对路径
   c. 调用 Resolve* 函数（放在独立 feature 文件中）
3. Resolve* 函数做：
   a. sourceContext() 获取 Doc + Lines + position encoding
   b. 在 Doc 结构上做语义查询
   c. 调用 index 方法做跨文件查询
   d. 返回 protocol.* 类型的结果
```

这个模式保持了清晰的关注点分离：`handler.go` 只做协议适配和参数提取，feature 文件包含语义逻辑，`index/` 提供数据访问。
