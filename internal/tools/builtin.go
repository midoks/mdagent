package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Builtin 返回框架内置工具列表。
func Builtin() []Tool {
	return []Tool{&timeTool{}, &calcTool{}, &echoTool{}}
}

// ---- get_time ----

type timeTool struct{}

func (t *timeTool) Name() string        { return "get_time" }
func (t *timeTool) Description() string { return "获取当前时间，可指定 IANA 时区（默认本地时区）" }
func (t *timeTool) Parameters() string {
	return `{"type":"object","properties":{"timezone":{"type":"string","description":"如 Asia/Shanghai，可省略"}}}`
}

func (t *timeTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Timezone string `json:"timezone"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return "", fmt.Errorf("参数解析失败: %w", err)
		}
	}
	loc := time.Local
	if in.Timezone != "" {
		l, err := time.LoadLocation(in.Timezone)
		if err != nil {
			return "", fmt.Errorf("无效时区 %q", in.Timezone)
		}
		loc = l
	}
	return time.Now().In(loc).Format("2006-01-02 15:04:05 MST"), nil
}

// ---- calculator ----

type calcTool struct{}

func (c *calcTool) Name() string        { return "calculator" }
func (c *calcTool) Description() string { return "计算四则运算表达式，支持 + - * / % 与括号" }
func (c *calcTool) Parameters() string {
	return `{"type":"object","properties":{"expression":{"type":"string","description":"算式，如 1+2*3"}},"required":["expression"]}`
}

func (c *calcTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	v, err := evalExpr(in.Expression)
	if err != nil {
		return "", err
	}
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10), nil
	}
	return strconv.FormatFloat(v, 'f', -1, 64), nil
}

// evalExpr 解析并计算四则运算表达式（递归下降实现，无外部依赖）。
func evalExpr(input string) (float64, error) {
	p := &exprParser{s: input}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos != len(p.s) {
		return 0, fmt.Errorf("表达式在位置 %d 处存在非法字符", p.pos)
	}
	return v, nil
}

type exprParser struct {
	s   string
	pos int
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.s) && p.s[p.pos] == ' ' {
		p.pos++
	}
}

func (p *exprParser) parseExpr() (float64, error) {
	v, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || (p.s[p.pos] != '+' && p.s[p.pos] != '-') {
			return v, nil
		}
		op := p.s[p.pos]
		p.pos++
		r, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			v += r
		} else {
			v -= r
		}
	}
}

func (p *exprParser) parseTerm() (float64, error) {
	v, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || !strings.ContainsRune("*/%", rune(p.s[p.pos])) {
			return v, nil
		}
		op := p.s[p.pos]
		p.pos++
		r, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		switch op {
		case '*':
			v *= r
		case '/':
			if r == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			v /= r
		case '%':
			if r == 0 {
				return 0, fmt.Errorf("取余除数不能为 0")
			}
			v = float64(int64(v) % int64(r))
		}
	}
}

func (p *exprParser) parseFactor() (float64, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return 0, fmt.Errorf("表达式不完整")
	}
	ch := p.s[p.pos]
	if ch == '-' {
		p.pos++
		v, err := p.parseFactor()
		return -v, err
	}
	if ch == '+' {
		p.pos++
		return p.parseFactor()
	}
	if ch == '(' {
		p.pos++
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		p.pos++
		return v, nil
	}
	start := p.pos
	for p.pos < len(p.s) && ((p.s[p.pos] >= '0' && p.s[p.pos] <= '9') || p.s[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("位置 %d 处期望数字", p.pos)
	}
	return strconv.ParseFloat(p.s[start:p.pos], 64)
}

// ---- echo ----

type echoTool struct{}

func (e *echoTool) Name() string        { return "echo" }
func (e *echoTool) Description() string { return "原样返回输入文本，用于连通性测试" }
func (e *echoTool) Parameters() string {
	return `{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`
}

func (e *echoTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	return in.Text, nil
}
