package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/memory"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

func TestDefaultFeaturesAllEnabled(t *testing.T) {
	features := DefaultFeatures()
	for _, name := range []string{"memory", "relevant_memory", "context_reduction", "prompt_cache"} {
		if !features.Enabled(name) {
			t.Fatalf("%s 默认应开启", name)
		}
	}
}

func TestFeaturesApplyOverrides(t *testing.T) {
	features := DefaultFeatures().Apply(map[string]bool{
		"memory":       false,
		"prompt_cache": false,
	})
	if features.Memory || features.PromptCache {
		t.Fatalf("覆盖未生效: %#v", features)
	}
	if !features.ContextReduction || !features.RelevantMemory {
		t.Fatalf("未列出的开关应保持开启: %#v", features)
	}
}

func TestMemoryOffSkipsMemorySection(t *testing.T) {
	store, _ := memory.Open(t.TempDir())
	store.RememberFile("auth.go", "登录校验", "1:1")

	model := &scriptedCompleter{responses: []string{"<final>ok</final>"}}
	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "p",
		UserInput: "看看",
		Memory:    store,
		Features:  DefaultFeatures().Apply(map[string]bool{"memory": false}),
	})

	if strings.Contains(model.prompts[0], "auth.go") {
		t.Fatal("关闭记忆后不应注入文件摘要")
	}
}

func TestPromptCacheOffSkipsCacheKey(t *testing.T) {
	recorder := &captureRecorder{}
	model := &scriptedCompleter{responses: []string{"<final>ok</final>"}}
	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "stable",
		UserInput: "hi",
		Recorder:  recorder,
		Features:  DefaultFeatures().Apply(map[string]bool{"prompt_cache": false}),
	})

	request, ok := recorder.find("model_requested")
	if !ok {
		t.Fatal("缺少请求事件")
	}
	if request.data["cache_key"] != "" {
		t.Fatalf("关闭缓存后不应带键: %v", request.data["cache_key"])
	}
}
