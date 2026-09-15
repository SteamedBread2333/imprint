# dsnu-agent skills → imprint（清空后重测）

日期：2026-09-11  
来源：`/Users/derbysoft-i129/Documents/Bitbucket/dsnu-agent`  
Vault：`./.imprint/memory/`（已 `imprint clear --confirm --yes`，压测库 `tests/loadvault` 已删）

不是把 SKILL.md 全文塞进 vault。每条 skill 收成 **祈使句 claim + 窄 scope + 原文证据**，用 `related` 连回 dispatcher。

复现：`go run ./tests/seed-dsnu`

## 召回抽检

| scope | 首条命中 | conf |
|---|---|---|
| `dsnu,thinking-process` | [r-2026-09-11-001] 先加载 thinking-process，额外最多一个 skill | 0.90 |
| `dsnu,page-implement` | [r-2026-09-11-011] 未选定 ingress 不写业务代码 | 0.90 |
| `dsnu,e2e-playwright` | [r-2026-09-11-008] 仅在操作者要求时跑 e2e，Loop A/B 之后不推销 | 0.90 |
| `dsnu,ui` | [r-2026-09-11-013] 只用 `@derbysoft/neat-design`，禁止 `antd` | 0.90 |

CLI `imprint find` / `imprint list` 可召回。

## Skill → 规则

| Skill / 文件 | 收成的规则 |
|---|---|
| thinking-process, AGENTS.md, SKILL_RULES.md | 001 调度、005 产品隔离、006 Loop A、007 Loop B、009 start 权限、010 context budget |
| 操作者界面 | 002 不暴露 skill 名、003 跟随会话语言、004 只用 AskQuestion |
| page-implement + composition/wiring | 011–021 页模块与 ingress；013–014 neat-design；015 layouts；016–018 封装；019 不编造契约；022–026 i18n/RBAC/HTTP |
| repo-review | 027 只审 diff、028 配对脚本、029 就地删死代码 |
| e2e-playwright | 008 按需、030 仅 Playwright MCP、031 不安装、032 先写完再跑、033 新 tab、034 locator、035 MCP 缺失只给短提醒 |
| 002_git_commit.mdc | 036 `{emoji} rationale`，禁止 `--no-verify` |
| README / CHECKLIST | 037 `npx dsnu-agent setup` 不挂 postinstall；038 禁止机器绝对路径 |

共 **40** 条 active（seed 当时），写在 `.imprint/memory/imprint-0001.md` 一个分片里，不再一规则一文件。图谱用 `imprint desk open`（desk 插件）或 `imprint viz --format mermaid`。点左侧 `page-implement` / `e2e-playwright` / `thinking-process` 看团。

## 刻意没写进 imprint 的

- 各 `references/*.md` 的逐步 runbook（那是 skill 正文，不是长期偏好）
- Playwright 安装教程全文
- neat-design 组件清单
- 消费者仓库里的具体页面路径
