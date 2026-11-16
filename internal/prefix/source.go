package prefix

import (
	"sync"
)

// Source 缓存提示词前缀，只在工作区指纹变化时重建。
type Source struct {
	mu          sync.Mutex
	build       func() string
	fingerprint func() string
	last        string
	text        string
	rebuilds    int
}

// New 构造前缀来源。fingerprint 决定是否需要重建。
func New(build func() string, fingerprint func() string) *Source {
	source := &Source{build: build, fingerprint: fingerprint}
	source.last = source.currentFingerprint()
	source.text = build()
	source.rebuilds = 1
	return source
}

// Get 返回前缀，工作区变化时先重建。
func (s *Source) Get() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.currentFingerprint()
	if current != s.last {
		s.text = s.build()
		s.last = current
		s.rebuilds++
	}
	return s.text
}

// Rebuilds 返回构建次数，用于观察是否真的只在变化时重建。
func (s *Source) Rebuilds() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rebuilds
}

func (s *Source) currentFingerprint() string {
	if s.fingerprint == nil {
		return ""
	}
	return s.fingerprint()
}
