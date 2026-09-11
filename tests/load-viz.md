# viz 意义与 3000 条压测

日期：2026-09-11

## 7 条时为什么看起来没意义

`memory/dashboard.html` 里那张图不是失败，是**规模不够、布局也不对**：

- 图要回答的是 PROMPT §22 的问题：supersede 链、scope 团、冲突。7 个点里只有 **一条** 红箭头（007 → 006）。
- 旧模板用 `cose` 把 5 个点撑到整块黑底上，标签还裁成 `2026-09-11-001`，看起来像空的装饰。
- 你当时 scope 滤成 `go`，003/005 被藏掉，更稀。

它**不是**「规则越多越好看的海报」。全量摊开几千个点会变成毛球，和 7 个点一样读不了。有用的交互是：左边看 scope 质量 → 点进一个 scope → 只图这一刀上的替换/关联。

已改模板（刷新 `memory/dashboard.html`）：

- ≤16 个点：圆布局，标签带 claim 摘要
- 全库 >40 且未过滤：**先总览**（scope 条、supersede 链列表），不画全图
- 过滤后的切片：最多画 300 个点（有边的优先，其余按 confidence）
- 「graph anyway」才强行画当前切片

## 3000 条压测

```bash
go run ./cmd/loadgen -n 3000 -vault tests/loadvault
open tests/loadvault/dashboard.html
```

合成数据：5 语言 × 8 主题，约每 12 条一条 supersede，另有 related / conflicts；dormant/superseded 进 `archive/`。写入走 `ImportRecords`，打进 `imprint-NNNN.md` 分片（32768 行 / 1 MiB 封顶），不再一规则一文件。

| 指标 | 结果 |
|---|---|
| 写入 | **3000** rules / **113ms**（2422 active, 110 dormant, 468 superseded） |
| 分片 | active `imprint-0001.md` 32763 行 + `imprint-0002.md` 16585 行；archive `imprint-0001.md` 12442 行 |
| `imprint viz --include-archived` | **69ms**，`tests/loadvault/dashboard.html` **1.16MB**，`rules_count=3000` |
| `find --scope go,naming --top-k 5` | **64ms**，5 hits（例：`r-2025-02-07-001` score 0.975） |

打开 loadvault 的 dashboard 时：默认应是 **3000 rules 总览 + scope 条**，不是 3000 个绿点。点左侧 `go` 或 `naming` 才会出图；图上应能看到 supersede 红箭头，而不是均匀撒点。

本机 vault `./memory/` 是打包后的 `imprint-0001.md`（真实规则，不是压测数据）。压测目录已 gitignore。

## 结论

| 规模 | 图在干什么 |
|---|---|
| 几条 | 几乎只能看见那一条替换链；用圆布局 + claim 标签才读得出来 |
| 几千条 | 全图画出来无意义；总览看分布，**按 scope 下钻**才是 viz 的工作 |

再压：`go run ./cmd/loadgen -n 8000 -vault tests/loadvault`（find/viz 随**规则数**近似线性；分片数量很少，HTML 大约每条几百字节）。
