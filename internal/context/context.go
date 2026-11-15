package context

import "strings"

// 各节名称。
const (
	SectionPrefix   = "prefix"
	SectionMemory   = "memory"
	SectionRelevant = "relevant"
	SectionHistory  = "history"
	SectionRequest  = "current_request"
)

// DefaultTotalBudget 是一轮 prompt 的总预算，单位为字符。
const DefaultTotalBudget = 12000

// DefaultBudgets 是各节的初始预算。
var DefaultBudgets = map[string]int{
	SectionPrefix:   3600,
	SectionMemory:   1600,
	SectionRelevant: 1200,
	SectionHistory:  5200,
}

// DefaultFloors 是各节的下限，收缩不会低于此值。
var DefaultFloors = map[string]int{
	SectionPrefix:   1200,
	SectionMemory:   400,
	SectionRelevant: 300,
	SectionHistory:  1500,
}

// DefaultReductionOrder 是超预算时的收缩顺序：先牺牲相关笔记，再历史，
// 最后才动记忆与前缀。当前请求不在其中，永不裁剪。
var DefaultReductionOrder = []string{SectionRelevant, SectionHistory, SectionMemory, SectionPrefix}

// Sections 是一轮 prompt 的原始素材。
type Sections struct {
	Prefix         string
	Memory         string
	Relevant       string
	History        string
	CurrentRequest string
}

// Options 允许覆盖默认预算与顺序。
type Options struct {
	TotalBudget    int
	Budgets        map[string]int
	Floors         map[string]int
	ReductionOrder []string
}

// SectionRender 记录一节在本次拼装中的长度变化。
type SectionRender struct {
	Raw      int
	Rendered int
	Budget   int
}

// Reduction 是一次收缩记录。
type Reduction struct {
	Section  string
	Before   int
	After    int
	Overflow int
}

// Result 是一次拼装的结果。
type Result struct {
	Prompt     string
	Sections   map[string]SectionRender
	Reductions []Reduction
}

// Build 按预算渲染并拼装一轮 prompt。同样的输入永远得到同样的输出。
func Build(sections Sections, options Options) Result {
	total := options.TotalBudget
	if total <= 0 {
		total = DefaultTotalBudget
	}
	budgets := merge(DefaultBudgets, options.Budgets)
	floors := merge(DefaultFloors, options.Floors)
	order := options.ReductionOrder
	if len(order) == 0 {
		order = DefaultReductionOrder
	}

	raw := map[string]string{
		SectionPrefix:   sections.Prefix,
		SectionMemory:   sections.Memory,
		SectionRelevant: sections.Relevant,
		SectionHistory:  sections.History,
		SectionRequest:  sections.CurrentRequest,
	}

	rendered := renderAll(raw, budgets, floors)
	prompt := assemble(rendered)
	reductions := []Reduction{}

	for len(prompt) > total {
		overflow := len(prompt) - total
		reduced := false
		for _, section := range order {
			budget := budgets[section]
			floor := floors[section]
			if budget <= floor {
				continue
			}
			next := budget - overflow
			if next < floor {
				next = floor
			}
			if next >= budget {
				continue
			}
			reductions = append(reductions, Reduction{
				Section:  section,
				Before:   budget,
				After:    next,
				Overflow: overflow,
			})
			budgets[section] = next
			rendered = renderAll(raw, budgets, floors)
			prompt = assemble(rendered)
			reduced = true
			break
		}
		if !reduced {
			break
		}
	}

	stats := make(map[string]SectionRender, len(raw))
	for section, text := range raw {
		stats[section] = SectionRender{
			Raw:      len(text),
			Rendered: len(rendered[section]),
			Budget:   budgets[section],
		}
	}

	return Result{Prompt: prompt, Sections: stats, Reductions: reductions}
}

func renderAll(raw map[string]string, budgets, floors map[string]int) map[string]string {
	out := map[string]string{}
	for section, text := range raw {
		if section == SectionRequest {
			out[section] = text
			continue
		}
		budget := budgets[section]
		if budget <= 0 {
			out[section] = text
			continue
		}
		out[section] = clip(text, budget, section == SectionPrefix)
	}
	return out
}

// clip 把文本压到上限内。前缀保留开头（规则在前），其余保留结尾（近期在后）。
func clip(text string, limit int, keepHead bool) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	if keepHead {
		return text[:limit-3] + "..."
	}
	return "..." + text[len(text)-(limit-3):]
}

func assemble(rendered map[string]string) string {
	var builder strings.Builder
	write := func(header, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		if header != "" {
			builder.WriteString(header)
			builder.WriteString("\n")
		}
		builder.WriteString(body)
	}

	write("", rendered[SectionPrefix])
	write("Memory:", rendered[SectionMemory])
	write("Relevant memory:", rendered[SectionRelevant])
	write("History:", rendered[SectionHistory])
	write("Current user request:", rendered[SectionRequest])
	return strings.TrimSpace(builder.String())
}

func merge(defaults, override map[string]int) map[string]int {
	out := make(map[string]int, len(defaults))
	for key, value := range defaults {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}
