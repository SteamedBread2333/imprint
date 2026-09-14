# 记忆纠偏

> **上手：** [README.zh.md](../README.zh.md#快速开始)。本文是**参考**（ADD / REINFORCE / SUPERSEDE / IGNORE、编码场景）。

用户纠正后偏好写入 `./memory/`，下次任务前召回。智能体 **`find` → 分类 → 写入**；用户正常说话，不维护规则 id。

## 谁维护 vault（无额外心智负担）

**用户**只在正常写代码、正常说话——纠正、确认、否定偏好即可，**不需要**知道 `imprint`、`forget`、规则 id 或任何维护口令。

**智能体**负责：`find`、分类、`add` / `reinforce` / `supersede` / `forget`；用户说「别记了」「那个不算了」「以后看 STYLE.md」时，由智能体查 vault 并执行，**不要**让用户复述命令或 id。

**人（可选）**想审计时再打开 `memory/dashboard.html` 或跑 `show` / `viz`——这是查阅，不是日常维护流程。

**Review + 剪枝（允许）**用户可以说「帮我看看命名相关的记忆，过时的砍掉」——智能体用 `show` / `viz` / `get` 展示链路与分支，用**自然语言**说明，用户确认后由智能体 `supersede` / `forget` / `sweep`。**禁止**的是让用户自己报 id 或跑命令，不是禁止整理 vault。

文档里的 CLI / MCP 示例是**智能体执行面**，不是要求用户背诵的操作手册。

## 纠偏循环

```mermaid
sequenceDiagram
  participant U as 用户
  participant A as 智能体
  participant I as imprint

  U->>A: 偏好 / 纠正 / 「不要那样做」
  A->>I: find（窄 scope，可选 query）
  I-->>A: 已有规则 ranked 列表

  alt 无相关规则
    A->>A: ADD
    A->>I: add
  else 同一句话再说一遍
    A->>A: REINFORCE
    A->>I: reinforce
  else 说法变了 / 范围变了 / 旧规则错了
    A->>A: SUPERSEDE
    A->>I: supersede
  else 用户说别记 / 一次性的
    A->>A: IGNORE 或 forget
  end

  Note over A,I: 下次写代码前再 find，引用 [r-id]
```

| 步骤 | 谁做 | 做什么 |
| --- | --- | --- |
| **召回** | 智能体 | `find --scope tag,tag`（标签 **AND**），必要时加 `--query` BM25 |
| **分类** | 智能体 | ADD / REINFORCE / SUPERSEDE / IGNORE（见下表） |
| **写入** | imprint | `add` / `reinforce` / `supersede` / `forget` |
| **审计** | 人 | `get`、`show`、`viz`；旧规则在 `archive/`，可追溯 |

优先 **MCP 工具**；未挂载时用 **`imprint --json`**（见 [mcp.zh.md](mcp.zh.md)）。

## 四种写入分类

| 分类 | 何时用 | 命令 | 对 vault 的影响 |
| --- | --- | --- | --- |
| **ADD** | `find` 无匹配；用户**新**偏好 | `add` | 新 `active` 规则，默认 confidence **0.6** |
| **REINFORCE** | 已有规则；用户**再次确认**同一偏好 | `reinforce` | confidence **+0.1**（上限 **0.95**），`reinforcement_count++`，可唤醒 `dormant` |
| **SUPERSEDE** | 偏好**改了**、范围**扩大/缩小**、旧 claim **不再成立** | `supersede` | 旧规则 → `superseded` 并进 `archive/`；新规则 `active`，链上 `supersedes: [old_id]` |
| **IGNORE** | 一次性指令、闲聊、智能体**推断**出的偏好 | （不写） | 无 |

额外：

| 操作 | 何时用 |
| --- | --- |
| **forget** | 用户用**日常口语**否定某条（「别记了」「那个不算」）；由智能体 `find` 后 `forget`，用户不提 imprint |
| **sweep** | 定期衰减长期未触达规则（默认 90 天 −0.05；低于 0.3 → `dormant` 归档） |

### 置信度约定（智能体侧）

写入 `add` 时可设 `--confidence` / MCP `confidence`：

| 用户原话强度 | 建议 confidence |
| --- | --- |
| 随口一提、首次偏好 | **0.6**（默认） |
| 明确纠正（「不对，应该…」） | **0.85** |
| 「从现在起永远…」 | **0.9** |

`reinforce` 在现有值上累加，不替代「纠正应 supersede + 高 confidence 新规则」。

### 只记用户原话

- **`--text` / `text`**：用户原话（evidence）
- **`claim`**：可执行的陈述句（给智能体读）
- 不要存密钥；不要推断用户没说的偏好

---

## 编码场景示例

以下用 **`./memory`** vault；MCP 与 CLI 等价，各举一种写法。

### 场景 1：新偏好 — ADD

**对话**

> 用户：Go 里 exported 标识符一律 PascalCase，别用 snake_case export。

**智能体**

1. `find --scope go,naming --query PascalCase` → 无命中  
2. 分类：**ADD**

```bash
imprint --json --vault ./memory add \
  "Go exported identifiers must use PascalCase" \
  --scope go,naming \
  --text "Go 里 exported 标识符一律 PascalCase，别用 snake_case export"
```

MCP `add`：`claim` / `scope` / `text` 同上，`confidence` 省略（0.6）。

**下次**：写 Go 导出符号前先 `find --scope go,naming`，命中则按 `[r-…]` 执行。

---

### 场景 2：再次确认 — REINFORCE

**对话**

> 用户：对，导出还是要 PascalCase，上次说的没错。

**智能体**

1. `find --scope go,naming` → 命中 `r-2026-09-14-001`  
2. 分类：**REINFORCE**（不是 ADD，避免重复）

```bash
imprint --json --vault ./memory reinforce r-2026-09-14-001 \
  --evidence "用户再次确认 exported 用 PascalCase"
```

MCP `reinforce`：`id` + `evidence`。

**结果**：confidence 0.6 → 0.7；`evidence_log` 多一条 `kind: reinforce`。

---

### 场景 3：纠正错误规则 — SUPERSEDE

**对话**

> 用户：不对，我们 internal 包可以用 unexported camelCase，只有 **跨包导出** 才要 PascalCase。

**智能体**

1. `find --scope go,naming` → 旧规则过宽  
2. 分类：**SUPERSEDE**（不是改旧文件手编辑，不是 ADD  duplicate）

```bash
imprint --json --vault ./memory supersede r-2026-09-14-001 \
  --claim "Go identifiers exported across packages must use PascalCase; internal unexported names use camelCase" \
  --scope go,naming \
  --reason "narrowed to cross-package exports only" \
  --text "不对，我们 internal 包可以用 unexported camelCase，只有跨包导出才要 PascalCase"
```

MCP `supersede`：`old_id`, `claim`, `scope`, `reason`, `text`。

**结果**：旧 id → `superseded` + `archive/`；新 id `active`，继承旧 confidence 并链到旧规则。Dashboard 上可看 **supersedes** 边。

---

### 场景 4：范围扩大 — SUPERSEDE

**对话**

> 用户：前端 TS 也一样，export 的组件和函数用 PascalCase / 同名约定，跟 Go 对齐。

**智能体**

1. `find --scope go,naming` 或 `typescript,naming`  
2. 分类：**SUPERSEDE**（scope 从 go → go + typescript）

```bash
imprint --json --vault ./memory supersede r-2026-09-14-002 \
  --claim "Exported Go and TypeScript symbols follow PascalCase (TS functions/components aligned with Go exports)" \
  --scope go,typescript,naming \
  --reason "extended naming rule to frontend TS" \
  --text "前端 TS 也一样，export 的组件和函数用 PascalCase，跟 Go 对齐"
```

---

### 场景 5：智能体差点推断 — IGNORE

**对话**

> 用户：帮我把这个 handler  refactor 一下，拆两个文件。

**智能体**

- 这是**任务指令**，不是长期偏好  
- `find` 可不写 vault  
- 分类：**IGNORE**

不写 `add`。若误存，用户随口「这个别记」→ 智能体 **`find` + `forget`**，用户无需知道 id。

---

### 场景 6：偏好作废 — 用户只说话，智能体收尾

**对话**

> 用户：命名以后以仓库里的 `STYLE.md` 为准，之前说的 PascalCase 导出约定不用了。

**智能体**

1. `find --scope go,naming` → 命中旧规则 `r-2026-09-14-003`  
2. 分类：**SUPERSEDE**（政策迁到 `STYLE.md`，保留审计链）或 **forget**（用户否定「别记这类命名偏好」时）  
3. **用户全程不说 imprint**；下面命令由智能体执行

**首选 supersede**（可追溯）：

```bash
imprint --json --vault ./memory supersede r-2026-09-14-003 \
  --claim "Follow STYLE.md for naming; imprint does not override the style guide" \
  --scope go,naming,docs \
  --reason "naming policy moved to STYLE.md" \
  --text "命名以后以仓库里的 STYLE.md 为准，之前说的 PascalCase 导出约定不用了"
```

**若用户是否定「不要再用 imprint 记命名」**（口语「别记这些了」）→ `forget`：

```bash
imprint --json --vault ./memory forget r-2026-09-14-003
```

MCP：同上，`supersede` 或 `forget`，参数由智能体从对话解析。

**回复用户**：确认已按 `STYLE.md` 执行即可，**不要**让用户确认规则 id 或选择命令。

---

### 场景 7：写代码前召回 — find

**对话**

> 用户：给 `UserService` 加个导出方法。

**智能体（编码前）**

```bash
imprint --json --vault ./memory find --scope go,naming --query export
```

命中 `[r-2026-09-14-002]` → 新方法名 `GetProfile` 而非 `get_profile`。在回复或 commit 说明中可 cite `[r-2026-09-14-002]`。

---

## 自然衰减 vs 主动纠偏

| 机制 | 触发 | 适用 |
| --- | --- | --- |
| **supersede / forget** | 用户日常口语纠正 | 规则**错了**或**作废**（智能体执行，用户不维护 vault） |
| **reinforce** | 用户重复确认 | 规则**仍对**，加强信心 |
| **sweep** | 运维 / 定期任务 | 长期未引用偏好**淡出**（默认 90 天未 touch −0.05；&lt; 0.3 → dormant） |

`sweep` 不会删规则内容；`dormant` 仍在 `archive/`，`reinforce` 可唤醒。

---

## 常用命令速查

```bash
# 写前召回
imprint --json --vault ./memory find --scope go,error-handling --query wrap

# 看单条证据链
imprint --json --vault ./memory get r-2026-09-14-002

# 高置信 active 规则
imprint --json --vault ./memory list --status active --scope go --min-confidence 0.85

# 全景
imprint --json --vault ./memory show
imprint --json --vault ./memory viz
```

---

## 参见

- [README.zh.md](../README.zh.md) — vault 布局与 CLI 全集  
- [mcp.zh.md](mcp.zh.md) — MCP 挂载与工具  
- [correction.md](correction.md) — English version  
- `.cursor/rules/imprint-memory.mdc` — 智能体必须遵守的 alwaysApply 规则
