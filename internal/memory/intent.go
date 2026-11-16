package memory

import (
	"regexp"
	"strings"
)

// intentPattern 匹配用户明确要求记住某件事的说法。
var intentPattern = regexp.MustCompile(`(?i)\b(capture|remember|save|store|persist|note)\b|记住|保存|记录|沉淀|长期记忆|持久记忆`)

// 行前缀决定这条内容属于哪个主题。
var lineTopics = []struct {
	pattern *regexp.Regexp
	topic   string
}{
	{regexp.MustCompile(`(?i)^(project convention|项目约定)[:：]\s*(.+)$`), TopicConventions},
	{regexp.MustCompile(`(?i)^(decision|决定)[:：]\s*(.+)$`), TopicDecisions},
	{regexp.MustCompile(`(?i)^(dependency|依赖)[:：]\s*(.+)$`), TopicDependencies},
	{regexp.MustCompile(`(?i)^(preference|偏好)[:：]\s*(.+)$`), TopicPreferences},
}

// HasMemoryIntent 判断用户是否在要求把内容记住。
func HasMemoryIntent(message string) bool {
	return intentPattern.MatchString(message)
}

// DetectPromotions 从用户消息里提取待提升的长期记忆。
// 没有明确的记住意图时返回空；有意图但行首没有主题标记时归入项目约定。
func DetectPromotions(message string) []Promotion {
	if !HasMemoryIntent(message) {
		return nil
	}
	promotions := make([]Promotion, 0, 4)
	fallback := make([]string, 0, 4)
	for _, raw := range strings.Split(message, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		matched := false
		for _, item := range lineTopics {
			if groups := item.pattern.FindStringSubmatch(line); groups != nil {
				promotions = append(promotions, Promotion{Topic: item.topic, Text: strings.TrimSpace(groups[len(groups)-1])})
				matched = true
				break
			}
		}
		if !matched && !isIntentOnly(line) {
			fallback = append(fallback, line)
		}
	}
	if len(promotions) > 0 {
		return promotions
	}
	for _, line := range fallback {
		if strings.TrimSpace(line) == "" {
			continue
		}
		promotions = append(promotions, Promotion{Topic: TopicConventions, Text: line})
	}
	return promotions
}

// isIntentOnly 判断这一行是不是只有「记住」这类动词，没有实际内容。
func isIntentOnly(line string) bool {
	remainder := intentPattern.ReplaceAllString(line, "")
	remainder = strings.TrimSpace(strings.Trim(remainder, "：:,，。."))
	return remainder == ""
}
