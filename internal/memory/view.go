package memory

import "strings"

// RenderView 把召回结果渲染成「相关笔记」一节。
// 短期笔记按召回顺序列出，长期记忆按主题分组附在后面。
func RenderView(notes []Note, durable []DurableNote) string {
	lines := make([]string, 0, len(notes)+len(durable)+2)
	for _, note := range notes {
		text := strings.TrimSpace(note.Text)
		if text == "" {
			continue
		}
		lines = append(lines, "- "+text)
	}

	grouped := map[string][]string{}
	order := make([]string, 0, 4)
	for _, note := range durable {
		text := strings.TrimSpace(note.Text)
		if text == "" {
			continue
		}
		if _, exists := grouped[note.Topic]; !exists {
			order = append(order, note.Topic)
		}
		grouped[note.Topic] = append(grouped[note.Topic], text)
	}
	for _, topic := range order {
		lines = append(lines, "- ["+topic+"]")
		for _, text := range grouped[topic] {
			lines = append(lines, "  - "+text)
		}
	}

	if len(lines) == 0 {
		return "Relevant memory:\n- none"
	}
	return "Relevant memory:\n" + strings.Join(lines, "\n")
}
