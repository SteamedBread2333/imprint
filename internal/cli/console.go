package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type lineState int

const (
	stateOK lineState = iota
	stateWarn
	stateOff
	stateFail
)

const labelWidth = 11

type console struct {
	w   io.Writer
	tty bool
}

func (a *App) console() *console {
	w := a.out()
	return newConsole(w)
}

func newConsole(w io.Writer) *console {
	tty := false
	if f, ok := w.(*os.File); ok {
		tty = isTerminal(f)
	}
	return &console{w: w, tty: tty}
}

func (c *console) Heading(title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	c.blank()
	if c.tty {
		fmt.Fprintf(c.w, "  \033[1m%s\033[0m\n", title)
	} else {
		fmt.Fprintf(c.w, "  %s\n", title)
	}
	fmt.Fprintf(c.w, "  %s\n\n", strings.Repeat("─", 42))
}

func (c *console) Blank() { c.blank() }

func (c *console) blank() {
	fmt.Fprintln(c.w)
}

func (c *console) KV(key, value string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if value == "" {
		value = "—"
	}
	fmt.Fprintf(c.w, "  %s  %s\n", c.label(key), value)
}

func (c *console) Row(name string, st lineState, badge, detail string) {
	glyph := "○"
	switch st {
	case stateOK:
		glyph = c.color("●", "32")
	case stateWarn:
		glyph = c.color("◐", "33")
	case stateFail:
		glyph = c.color("✕", "31")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = " "
	}
	badge = strings.TrimSpace(badge)
	detail = strings.TrimSpace(detail)
	if badge != "" {
		badge = c.badge(badge, st)
	}
	switch {
	case badge != "" && detail != "":
		fmt.Fprintf(c.w, "  %s  %s  %-10s  %s\n", c.label(name), glyph, badge, detail)
	case badge != "":
		fmt.Fprintf(c.w, "  %s  %s  %s\n", c.label(name), glyph, badge)
	case detail != "":
		fmt.Fprintf(c.w, "  %s  %s  %s\n", c.label(name), glyph, detail)
	default:
		fmt.Fprintf(c.w, "  %s  %s\n", c.label(name), glyph)
	}
}

func (c *console) Action(why, cmd string) {
	why = strings.TrimSpace(why)
	cmd = strings.TrimSpace(cmd)
	if why == "" || cmd == "" {
		return
	}
	c.blank()
	arrow := c.color("→", "2")
	fmt.Fprintf(c.w, "  %s  %s\n", arrow, why)
	fmt.Fprintf(c.w, "      %s\n", cmd)
}

func (c *console) Note(format string, args ...any) {
	c.dim(fmt.Sprintf("      %s", fmt.Sprintf(format, args...)))
}

func (c *console) Tip(format string, args ...any) {
	c.Action("next", fmt.Sprintf(format, args...))
}

func (c *console) Empty(msg string) {
	c.dim("      " + strings.TrimSpace(msg))
}

func (c *console) Done(format string, args ...any) {
	mark := c.color("✓", "32")
	fmt.Fprintf(c.w, "  %s  %s\n", mark, fmt.Sprintf(format, args...))
}

func (c *console) Error(msg string) {
	c.Row("", stateFail, "failed", msg)
}

func (c *console) RuleFind(id string, score, conf float64, scope, claim string) {
	meta := fmt.Sprintf("%s  ·  score %.2f  ·  conf %.2f", id, score, conf)
	if scope != "" {
		meta += "  ·  " + scope
	}
	fmt.Fprintf(c.w, "      %s\n", meta)
	c.ruleClaim(claim)
}

func (c *console) RuleBrief(id string, conf float64, status, scope, claim string) {
	meta := fmt.Sprintf("%s  ·  conf %.2f", id, conf)
	if status != "" {
		meta += "  ·  " + status
	}
	if scope != "" {
		meta += "  ·  " + scope
	}
	fmt.Fprintf(c.w, "      %s\n", meta)
	c.ruleClaim(claim)
}

func (c *console) ruleClaim(claim string) {
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return
	}
	for _, line := range strings.Split(claim, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		c.dim("               " + line)
	}
}

func (c *console) Section(title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	c.blank()
	if c.tty {
		fmt.Fprintf(c.w, "  \033[1m%s\033[0m\n", title)
	} else {
		fmt.Fprintf(c.w, "  %s\n", title)
	}
}

func (c *console) dim(s string) {
	if c.tty {
		fmt.Fprintf(c.w, "\033[2m%s\033[0m\n", s)
		return
	}
	fmt.Fprintln(c.w, s)
}

func (c *console) color(s, code string) string {
	if !c.tty {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func (c *console) badge(text string, st lineState) string {
	if !c.tty {
		return text
	}
	switch st {
	case stateOK:
		return c.color(text, "32")
	case stateWarn:
		return c.color(text, "33")
	case stateFail:
		return c.color(text, "31")
	default:
		return c.color(text, "2")
	}
}

func (c *console) label(key string) string {
	if len(key) >= labelWidth {
		return key
	}
	return key + strings.Repeat(" ", labelWidth-len(key))
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
