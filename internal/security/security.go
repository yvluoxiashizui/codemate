package security

import (
	"os"
	"sort"
	"strings"
)

// secretMarkers 是变量名里出现就按密钥处理的关键词。
var secretMarkers = []string{
	"KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "AUTH", "PRIVATE",
}

// minSecretLen 是参与打码的最短值长度，过短的值替换会误伤普通文本。
const minSecretLen = 8

// defaultShellEnv 是子进程默认继承的环境变量。
// 只放运行命令必需的白名单，密钥类变量一律不传。
var defaultShellEnv = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "LC_ALL", "TERM", "TMPDIR", "TZ", "PWD",
	"GOPATH", "GOROOT", "GOCACHE", "GOMODCACHE", "GOPROXY", "GOFLAGS",
}

// IsSecretEnvName 判断一个环境变量名是不是密钥。
func IsSecretEnvName(name string, extra []string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if upper == "" {
		return false
	}
	for _, item := range extra {
		if strings.ToUpper(strings.TrimSpace(item)) == upper {
			return true
		}
	}
	for _, marker := range secretMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

// Secrets 返回当前环境里被判定为密钥的变量，按值长度从长到短排序，
// 保证替换时长的值先被处理，避免短值把长值切碎。
func Secrets(extra []string) [][2]string {
	pairs := make([][2]string, 0, 8)
	for _, item := range os.Environ() {
		name, value, found := strings.Cut(item, "=")
		if !found || len(value) < minSecretLen {
			continue
		}
		if !IsSecretEnvName(name, extra) {
			continue
		}
		pairs = append(pairs, [2]string{name, value})
	}
	sort.Slice(pairs, func(i, j int) bool { return len(pairs[i][1]) > len(pairs[j][1]) })
	return pairs
}

// Redact 把文本里出现的密钥值替换成占位符。
func Redact(text string, extra []string) string {
	if text == "" {
		return text
	}
	for _, pair := range Secrets(extra) {
		text = strings.ReplaceAll(text, pair[1], "[redacted:"+pair[0]+"]")
	}
	return text
}

// RedactValue 递归处理可序列化的值，把字符串里的密钥值打码。
func RedactValue(value any, extra []string) any {
	switch typed := value.(type) {
	case string:
		return Redact(typed, extra)
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = RedactValue(item, extra)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = RedactValue(item, extra)
		}
		return out
	case []string:
		out := make([]string, len(typed))
		for index, item := range typed {
			out[index] = Redact(item, extra)
		}
		return out
	default:
		return value
	}
}

// ShellEnv 返回子进程应当继承的环境变量：只包含白名单内的名字，且排除密钥。
func ShellEnv(extraNames []string, extraEnv []string) []string {
	allowed := make(map[string]bool, len(defaultShellEnv)+len(extraNames))
	for _, name := range defaultShellEnv {
		allowed[name] = true
	}
	for _, name := range extraNames {
		allowed[strings.ToUpper(strings.TrimSpace(name))] = true
	}

	out := make([]string, 0, len(allowed))
	for _, item := range os.Environ() {
		name, value, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		upper := strings.ToUpper(name)
		if !allowed[upper] {
			continue
		}
		if IsSecretEnvName(name, extraEnv) {
			continue
		}
		out = append(out, name+"="+value)
	}
	sort.Strings(out)
	return out
}
