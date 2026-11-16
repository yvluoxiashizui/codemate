package memory

import "testing"

func TestHasMemoryIntent(t *testing.T) {
	cases := map[string]bool{
		"记住：所有错误都要包装":          true,
		"please remember this": true,
		"把这行保存下来":              true,
		"查一下这个函数的调用链":          false,
		"fix the bug":          false,
	}
	for message, want := range cases {
		if got := HasMemoryIntent(message); got != want {
			t.Fatalf("%q = %v, want %v", message, got, want)
		}
	}
}

func TestDetectPromotionsByTopicPrefix(t *testing.T) {
	message := "记住这两条\nDecision: 数据库用 mysql 8\nPreference: 回答保持简洁"
	promotions := DetectPromotions(message)

	if len(promotions) != 2 {
		t.Fatalf("promotions = %#v", promotions)
	}
	if promotions[0].Topic != TopicDecisions || promotions[1].Topic != TopicPreferences {
		t.Fatalf("主题识别错误: %#v", promotions)
	}
}

func TestDetectPromotionsFallbackToConventions(t *testing.T) {
	promotions := DetectPromotions("记住：统一用 gofmt 格式化")
	if len(promotions) != 1 {
		t.Fatalf("promotions = %#v", promotions)
	}
	if promotions[0].Topic != TopicConventions {
		t.Fatalf("无主题标记应归入项目约定: %s", promotions[0].Topic)
	}
	if promotions[0].Text == "" {
		t.Fatal("内容不应为空")
	}
}

func TestDetectPromotionsWithoutIntent(t *testing.T) {
	if got := DetectPromotions("帮我看下这个文件"); len(got) != 0 {
		t.Fatalf("没有记住意图时不应提升: %#v", got)
	}
}

func TestDetectPromotionsMatchesChinesePrefix(t *testing.T) {
	promotions := DetectPromotions("记住\n决定：缓存用 redis")
	if len(promotions) != 1 {
		t.Fatalf("promotions = %#v", promotions)
	}
	if promotions[0].Topic != TopicDecisions {
		t.Fatalf("中文冒号应被识别: %s", promotions[0].Topic)
	}
}
