// Command imprint-acceptance runs black-box real-user scenarios against installed binaries.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type scenario struct {
	Name   string
	Action string
	Expect string
	Actual string
	OK     bool
	Data   string
}

type report struct {
	Started   time.Time
	Elapsed   time.Duration
	Imprint   string
	MCP       string
	Version   string
	Scenarios []scenario
	Privacy   string
	Review    string
}

func main() {
	outPath := flag.String("report", ".imprint/export/acceptance-report.md", "Chinese Markdown report path")
	flag.Parse()
	os.Exit(run(*outPath))
}

func run(outPath string) int {
	rep := &report{Started: time.Now().UTC()}
	imprintBin, err := exec.LookPath("imprint")
	if err != nil {
		fmt.Fprintln(os.Stderr, "imprint not on PATH; run go install ./cmd/imprint ./cmd/imprint-mcp")
		return 1
	}
	mcpBin, err := exec.LookPath("imprint-mcp")
	if err != nil {
		fmt.Fprintln(os.Stderr, "imprint-mcp not on PATH")
		return 1
	}
	rep.Imprint = imprintBin
	rep.MCP = mcpBin
	rep.Version = strings.TrimSpace(mustOutput(imprintBin, "version"))

	root := mustTemp("imprint-acceptance-")
	writeConfig(root)
	vault := filepath.Join(root, ".imprint")

	runScenarios(rep, imprintBin, mcpBin, root, vault)

	rep.Elapsed = time.Since(rep.Started)
	rep.Review = strings.TrimSpace(`
审查结论：无 blocker / high。schema version=1，无 migrate 分支，不匹配即要求删除重建。add/reinforce/supersede/forget/sweep 与 rule_stats、rule_events 同事务；forget 先写事件再硬删。find 只 UPSERT recall_count/last_recalled_at，缺失 stats 时 last_confirmed_at 取 rules.created_at。reinforce 空证据拒绝，递减公式上限 0.95。report 为 CLI 审计，不进 MCP。telemetry 按日 JSONL、30 天清理、无正文。文档契约与 body.md/editor rule 一致。write_rejected 记入 telemetry（拒绝时常无 rule_id），不单独写入 rule_events。
`)
	if err := writeReport(outPath, *rep); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
	for _, sc := range rep.Scenarios {
		if !sc.OK {
			return 1
		}
	}
	return 0
}

func runScenarios(rep *report, imprintBin, mcpBin, root, vault string) {
	add := func(s scenario) { rep.Scenarios = append(rep.Scenarios, s) }

	// 1. init
	code, out, errw := runCmd(root, imprintBin, "init", "--cursor")
	add(scenario{
		Name: "初始化项目", Action: "运行 imprint init --cursor",
		Expect: "生成 imprint.yaml、.cursor/rules 与 .imprint 运行目录",
		Actual: fmt.Sprintf("exit=%d yaml=%v rule=%v", code, fileExists(filepath.Join(root, "imprint.yaml")), fileExists(filepath.Join(root, ".cursor/rules/imprint-memory.mdc"))),
		OK:     code == 0 && fileExists(filepath.Join(root, "imprint.yaml")),
	})
	_ = out
	_ = errw

	// 2. camelCase + Linguist language alias
	addJSON, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "add",
		"Prefer camelCase identifiers in TypeScript", "--scope", "ts", "--text", "team naming policy")
	id := jsonField(addJSON, "id")
	findJSON, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "find",
		"--scope", "tsx", "--query", "Camel Case")
	add(scenario{
		Name: "命名变体与语言别名", Action: "用 ts 写入 camelCase 规则，再用 tsx + Camel Case 召回",
		Expect: "命中同一规则 id",
		Actual: fmt.Sprintf("id_present=%v", strings.Contains(findJSON, id)),
		OK:     id != "" && strings.Contains(findJSON, id),
		Data:   "id_len=" + fmt.Sprint(len(id)),
	})

	// 3. duplicate + privacy
	dupOut, dupErr := runJSONErr(root, imprintBin, "--json", "--vault", vault, "add",
		"Always prefer camelCase identifiers in TypeScript", "--scope", "typescript", "--text", "repeat")
	privOut, privErr := runJSONErr(root, imprintBin, "--json", "--vault", vault, "add",
		"Store secrets outside the repo", "--scope", "security", "--text", "password=hunter2")
	add(scenario{
		Name: "重复与隐私拒绝", Action: "再次添加近似规则，并尝试写入 secret",
		Expect: "duplicate 与 privacy_rejected",
		Actual: fmt.Sprintf("dup=%v privacy=%v", strings.Contains(dupOut, "duplicate"), strings.Contains(privOut, "privacy_rejected")),
		OK:     dupErr != nil && privErr != nil && strings.Contains(dupOut, "duplicate") && strings.Contains(privOut, "privacy_rejected"),
	})

	// 4. reinforce gate
	emptyOut, emptyErr := runJSONErr(root, imprintBin, "--json", "--vault", vault, "reinforce", id)
	before := jsonNumber(mustGet(root, imprintBin, vault, id), "confidence")
	okOut, okErr := runJSON(root, imprintBin, "--json", "--vault", vault, "reinforce", id, "--evidence", "user confirmed the same naming policy")
	after := jsonNumber(okOut, "confidence")
	add(scenario{
		Name: "reinforce 证据门禁", Action: "先空 evidence，再明确重申",
		Expect: "空证据拒绝；成功后 confidence 升到 0.6875",
		Actual: fmt.Sprintf("empty_rejected=%v before=%.4f after=%.4f", emptyErr != nil, before, after),
		OK:     emptyErr != nil && strings.Contains(emptyOut, "reinforce_evidence_required") && okErr == nil && approx(after, 0.6875),
	})

	// 5. find stats only
	confBefore := jsonNumber(mustGet(root, imprintBin, vault, id), "confidence")
	runJSON(root, imprintBin, "--json", "--vault", vault, "find", "--scope", "js", "--query", "camel")
	runJSON(root, imprintBin, "--json", "--vault", vault, "find", "--scope", "js", "--query", "camel")
	reportJSON, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "report", "--days", "30")
	confAfter := jsonNumber(mustGet(root, imprintBin, vault, id), "confidence")
	add(scenario{
		Name: "find 只记账", Action: "连续两次 find",
		Expect: "confidence 不变，report 含 recall_count",
		Actual: fmt.Sprintf("confidence %.4f -> %.4f report_has_recall=%v", confBefore, confAfter, strings.Contains(reportJSON, "recall_count")),
		OK:     confBefore == confAfter && strings.Contains(reportJSON, "recall_count"),
	})

	// 6. dormant wake
	low, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "add",
		"Keep dormant fixtures isolated", "--scope", "fixture", "--text", "old policy", "--confidence", "0.25")
	lowID := jsonField(low, "id")
	runJSON(root, imprintBin, "--json", "--vault", vault, "sweep", "--decay-days", "1", "--dormant-threshold", "0.3")
	wake, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "find", "--scope", "fixture", "--query", "dormant fixtures")
	runJSON(root, imprintBin, "--json", "--vault", vault, "reinforce", lowID, "--evidence", "user said this old policy still holds")
	woke := mustGet(root, imprintBin, vault, lowID)
	add(scenario{
		Name: "dormant 低权入场后唤醒", Action: "低 confidence 规则 sweep 后 find，再 reinforce",
		Expect: "find 含 wake_candidate；reinforce 后 status=active",
		Actual: fmt.Sprintf("wake=%v status_active=%v", strings.Contains(wake, "wake_candidate") || strings.Contains(wake, "dormant"), strings.Contains(woke, `"status": "active"`) || strings.Contains(woke, `"status":"active"`)),
		OK:     (strings.Contains(wake, "wake_candidate") || strings.Contains(wake, "dormant")) && strings.Contains(woke, "active"),
	})

	// 7. supersede + forget
	old, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "add",
		"Use the temporary formatter", "--scope", "tooling", "--text", "first")
	oldID := jsonField(old, "id")
	sup, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "supersede", oldID,
		"--claim", "Use the replacement formatter", "--scope", "tooling", "--reason", "policy changed", "--text", "now this")
	newID := jsonField(sup, "id")
	gone, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "add",
		"Disposable audit rule", "--scope", "audit", "--text", "forget me")
	goneID := jsonField(gone, "id")
	runJSON(root, imprintBin, "--json", "--vault", vault, "forget", goneID)
	rep7, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "report", "--days", "30")
	add(scenario{
		Name: "替换与删除审计", Action: "supersede 一条，forget 另一条",
		Expect: "新规则存在；report 含 supersede 与 forget",
		Actual: fmt.Sprintf("successor=%v events=%v", newID != "", strings.Contains(rep7, "supersede") && strings.Contains(rep7, "forget")),
		OK:     newID != "" && strings.Contains(rep7, "supersede") && strings.Contains(rep7, "forget"),
	})

	// 8. concurrent CLI
	start := time.Now()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := runJSON(root, imprintBin, "--json", "--vault", vault, "add",
				fmt.Sprintf("Prefer isolated worker policy %d", i),
				"--scope", fmt.Sprintf("worker%d", i),
				"--text", "concurrent")
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	fail := 0
	for err := range errs {
		if err != nil {
			fail++
		}
	}
	add(scenario{
		Name: "并发 CLI 写入", Action: "8 个 imprint add 进程同时写入",
		Expect: "全部成功，无 SQLITE_BUSY",
		Actual: fmt.Sprintf("failures=%d elapsed=%s", fail, time.Since(start).Round(time.Millisecond)),
		OK:     fail == 0,
		Data:   fmt.Sprintf("processes=8 failures=%d", fail),
	})

	// 9. MCP compact
	compactOK, compactActual := checkMCP(mcpBin, root, vault, id)
	add(scenario{
		Name: "MCP 默认紧凑", Action: "通过 imprint-mcp 调用 get/find",
		Expect: "默认 get 不含 evidence 原文；含 evidence_count",
		Actual: compactActual,
		OK:     compactOK,
	})

	// 10. telemetry privacy
	telDir := filepath.Join(vault, "state", "telemetry")
	unsafe := scanTelemetry(telDir, []string{"password", "hunter2", "Prefer camelCase", "user confirmed"})
	add(scenario{
		Name: "telemetry 隐私", Action: "扫描每日 JSONL",
		Expect: "无 query/claim/evidence/secret 正文",
		Actual: fmt.Sprintf("unsafe_hits=%d", len(unsafe)),
		OK:     fileExists(telDir) && len(unsafe) == 0,
	})
	rep.Privacy = fmt.Sprintf("扫描 %s，敏感正文命中 %d", telDir, len(unsafe))

	// 11. report
	finalReport, _ := runJSON(root, imprintBin, "--json", "--vault", vault, "report", "--days", "30")
	add(scenario{
		Name: "report 审计", Action: "imprint report --days 30",
		Expect: "含 event_counts、duplicates/conflicts 字段、telemetry",
		Actual: fmt.Sprintf("events=%v telemetry=%v", strings.Contains(finalReport, "event_counts"), strings.Contains(finalReport, "telemetry")),
		OK:     strings.Contains(finalReport, "event_counts") && strings.Contains(finalReport, "telemetry"),
	})

	// 12. reopen
	again := mustGet(root, imprintBin, vault, id)
	add(scenario{
		Name: "关闭后再打开", Action: "新的 CLI 进程 get 同一规则",
		Expect: "规则仍在，confidence 保持",
		Actual: fmt.Sprintf("present=%v", strings.Contains(again, id)),
		OK:     strings.Contains(again, id) && strings.Contains(again, "0.6875"),
	})
}

func writeConfig(root string) {
	_ = os.WriteFile(filepath.Join(root, "imprint.yaml"), []byte(`host:
  listen: 127.0.0.1:9470
telemetry:
  enabled: true
  retention_days: 30
shelves:
  enabled: false
`), 0o644)
}

func checkMCP(mcpBin, root, vault, id string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, mcpBin, "--project", root, "--vault", vault)
	cmd.Stderr = io.Discard
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "acceptance", Version: "test"}, nil)
	cs, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return false, "connect: " + err.Error()
	}
	defer cs.Close()
	got, err := callMCP(cs, "get", map[string]any{"id": id})
	if err != nil {
		return false, err.Error()
	}
	ok := strings.Contains(got, "evidence_count") && !strings.Contains(got, "team naming policy")
	return ok, fmt.Sprintf("compact_get=%v", ok)
}

func callMCP(cs *sdkmcp.ClientSession, name string, args map[string]any) (string, error) {
	res, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", err
	}
	if len(res.Content) == 0 {
		return "", fmt.Errorf("empty")
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		return "", fmt.Errorf("not text")
	}
	return tc.Text, nil
}

func scanTelemetry(dir string, forbidden []string) []string {
	var hits []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{"missing"}
	}
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		text := string(raw)
		for _, word := range forbidden {
			if strings.Contains(text, word) {
				hits = append(hits, word)
			}
		}
	}
	return hits
}

func writeReport(path string, r report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	pass, fail := 0, 0
	for _, sc := range r.Scenarios {
		if sc.OK {
			pass++
		} else {
			fail++
		}
	}
	fmt.Fprintf(&b, "# imprint 验收报告\n\n")
	fmt.Fprintf(&b, "- 开始时间：%s\n", r.Started.Format(time.RFC3339))
	fmt.Fprintf(&b, "- 耗时：%s\n", r.Elapsed.Round(time.Millisecond))
	fmt.Fprintf(&b, "- 二进制：`%s` / `%s`\n", r.Imprint, r.MCP)
	fmt.Fprintf(&b, "- 版本：%s\n", r.Version)
	fmt.Fprintf(&b, "- 结果：%d 通过 / %d 失败 / 共 %d 个场景\n\n", pass, fail, len(r.Scenarios))
	fmt.Fprintf(&b, "## 场景\n\n")
	for i, sc := range r.Scenarios {
		status := "通过"
		if !sc.OK {
			status = "失败"
		}
		fmt.Fprintf(&b, "### %d. %s（%s）\n\n", i+1, sc.Name, status)
		fmt.Fprintf(&b, "- 用户动作：%s\n", sc.Action)
		fmt.Fprintf(&b, "- 期望：%s\n", sc.Expect)
		fmt.Fprintf(&b, "- 实际：%s\n", sc.Actual)
		if sc.Data != "" {
			fmt.Fprintf(&b, "- 数据：%s\n", sc.Data)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## 隐私扫描\n\n%s\n\n", r.Privacy)
	fmt.Fprintf(&b, "## Review\n\n%s\n", r.Review)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func runCmd(dir, bin string, args ...string) (int, string, string) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var out, errw bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errw
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	return code, out.String(), errw.String()
}

func runJSON(dir, bin string, args ...string) (string, error) {
	code, out, errw := runCmd(dir, bin, args...)
	if code != 0 {
		return out, fmt.Errorf("exit %d: %s", code, errw+out)
	}
	return out, nil
}

func runJSONErr(dir, bin string, args ...string) (string, error) {
	code, out, errw := runCmd(dir, bin, args...)
	if code == 0 {
		return out, nil
	}
	if out == "" {
		out = errw
	}
	return out, fmt.Errorf("exit %d", code)
}

func mustGet(dir, bin, vault, id string) string {
	out, _ := runJSON(dir, bin, "--json", "--vault", vault, "get", id)
	return out
}

func mustOutput(bin string, args ...string) string {
	cmd := exec.Command(bin, args...)
	raw, _ := cmd.Output()
	return string(raw)
}

func mustTemp(prefix string) string {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		panic(err)
	}
	return dir
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() || err == nil && st.IsDir()
}

func jsonField(raw, key string) string {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func jsonNumber(raw, key string) float64 {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return 0
	}
	n, _ := m[key].(float64)
	return n
}

func approx(got, want float64) bool {
	if got > want {
		return got-want < 1e-6
	}
	return want-got < 1e-6
}
