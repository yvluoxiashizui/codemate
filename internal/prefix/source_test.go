package prefix

import "testing"

func TestGetBuildsOnceWhenFingerprintStable(t *testing.T) {
	builds := 0
	source := New(
		func() string { builds++; return "prefix" },
		func() string { return "same" },
	)

	source.Get()
	source.Get()

	if builds != 1 {
		t.Fatalf("指纹不变时应只构建一次: %d", builds)
	}
	if source.Rebuilds() != 1 {
		t.Fatalf("构建次数 = %d", source.Rebuilds())
	}
}

func TestGetRebuildsWhenFingerprintChanges(t *testing.T) {
	builds := 0
	fingerprint := "first"
	source := New(
		func() string { builds++; return "prefix-" + fingerprint },
		func() string { return fingerprint },
	)

	first := source.Get()
	fingerprint = "second"
	second := source.Get()

	if builds != 2 {
		t.Fatalf("指纹变化应重建: %d", builds)
	}
	if first == second {
		t.Fatalf("重建后内容应变化: %q %q", first, second)
	}
}

func TestNilSourceGet(t *testing.T) {
	var source *Source
	if source.Get() != "" {
		t.Fatal("nil 来源应返回空串")
	}
	if source.Rebuilds() != 0 {
		t.Fatal("nil 来源构建次数为 0")
	}
}
