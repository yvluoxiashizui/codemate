package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Signature 返回工具集的签名：名称、说明与参数一起排序后取哈希。
// 增删工具或改参数都会改变签名，用于判断检查点是否还适用。
func (r *Registry) Signature() string {
	if r == nil {
		return ""
	}
	definitions := r.List()
	parts := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		params := make([]string, 0, 8)
		if properties, ok := definition.InputSchema["properties"].(map[string]any); ok {
			for name := range properties {
				params = append(params, name)
			}
		}
		sort.Strings(params)
		parts = append(parts, strings.Join([]string{
			definition.Name,
			definition.Description,
			strings.Join(params, ","),
		}, "|"))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:8])
}
