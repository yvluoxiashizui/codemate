package config

import (
	"os"
	"path/filepath"
	"strings"
)

// FindDotEnv 从起始目录向上找 .env，找不到时返回空串。
func FindDotEnv(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ParseDotEnv 读取 KEY=VALUE 形式的环境文件。
// 忽略空行与注释，去掉成对的引号，不展开变量。
func ParseDotEnv(path string) (map[string]string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, raw := range strings.Split(string(payload), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = stripQuotes(strings.TrimSpace(value))
	}
	return out, nil
}

func stripQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// DotEnvValues 返回从 start 向上找到的 .env 内容。
func DotEnvValues(start string) map[string]string {
	path := FindDotEnv(start)
	if path == "" {
		return nil
	}
	values, err := ParseDotEnv(path)
	if err != nil {
		return nil
	}
	return values
}
