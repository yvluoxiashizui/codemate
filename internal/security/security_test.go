package security

import (
	"strings"
	"testing"
)

func TestIsSecretEnvNameByMarker(t *testing.T) {
	cases := map[string]bool{
		"OPENAI_API_KEY": true,
		"GITHUB_TOKEN":   true,
		"DB_PASSWORD":    true,
		"AWS_SECRET":     true,
		"PATH":           false,
		"HOME":           false,
		"MAX_STEPS":      false,
	}
	for name, want := range cases {
		if got := IsSecretEnvName(name, nil); got != want {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestIsSecretEnvNameWithExtraList(t *testing.T) {
	if !IsSecretEnvName("MY_COMPANY_VAR", []string{"my_company_var"}) {
		t.Fatal("追加名单应不区分大小写生效")
	}
	if IsSecretEnvName("PATH", []string{"OTHER"}) {
		t.Fatal("不在名单里的名字不应误判")
	}
}

func TestRedactReplacesSecretValue(t *testing.T) {
	t.Setenv("CODEMATE_TEST_TOKEN", "supersecretvalue123")

	got := Redact("token is supersecretvalue123 here", nil)
	if strings.Contains(got, "supersecretvalue123") {
		t.Fatalf("密钥值未被替换: %q", got)
	}
	if !strings.Contains(got, "[redacted:CODEMATE_TEST_TOKEN]") {
		t.Fatalf("应带变量名占位符: %q", got)
	}
}

func TestRedactLeavesShortValuesAlone(t *testing.T) {
	t.Setenv("CODEMATE_TEST_KEY", "abc")

	got := Redact("abc is a common substring", nil)
	if !strings.Contains(got, "abc") {
		t.Fatalf("过短的值不应替换: %q", got)
	}
}

func TestRedactValueWalksNestedStructures(t *testing.T) {
	t.Setenv("CODEMATE_TEST_SECRET", "nestedsecretvalue")

	value := map[string]any{
		"note": "nestedsecretvalue",
		"list": []any{"nestedsecretvalue", 42},
	}
	redacted, ok := RedactValue(value, nil).(map[string]any)
	if !ok {
		t.Fatal("应返回同类型")
	}
	if strings.Contains(redacted["note"].(string), "nestedsecretvalue") {
		t.Fatalf("map 值未打码: %#v", redacted)
	}
	list := redacted["list"].([]any)
	if strings.Contains(list[0].(string), "nestedsecretvalue") {
		t.Fatalf("切片值未打码: %#v", list)
	}
	if list[1].(int) != 42 {
		t.Fatalf("非字符串值应原样保留: %#v", list[1])
	}
}

func TestShellEnvKeepsWhitelistAndDropsSecrets(t *testing.T) {
	t.Setenv("CODEMATE_TEST_TOKEN", "shouldnotleak12345")
	t.Setenv("CODEMATE_UNLISTED", "not-whitelisted")

	env := ShellEnv(nil, nil)
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "shouldnotleak12345") {
		t.Fatal("密钥不应传给子进程")
	}
	if strings.Contains(joined, "CODEMATE_UNLISTED") {
		t.Fatal("白名单外的变量不应传给子进程")
	}
	if !strings.HasPrefix(joined, "HOME=") && !strings.Contains(joined, "\nHOME=") {
		t.Fatalf("白名单内的 HOME 应保留: %s", joined)
	}
}

func TestShellEnvAcceptsExtraNames(t *testing.T) {
	t.Setenv("CODEMATE_EXTRA", "value")

	env := ShellEnv([]string{"CODEMATE_EXTRA"}, nil)
	if !strings.Contains(strings.Join(env, "\n"), "CODEMATE_EXTRA=value") {
		t.Fatal("追加的白名单变量应被保留")
	}
}

func TestShellEnvFiltersSecretEvenIfWhitelisted(t *testing.T) {
	t.Setenv("CODEMATE_API_KEY", "secretvaluehere")

	env := ShellEnv([]string{"CODEMATE_API_KEY"}, nil)
	if strings.Contains(strings.Join(env, "\n"), "secretvaluehere") {
		t.Fatal("即使进了白名单，密钥也不应传出")
	}
}
