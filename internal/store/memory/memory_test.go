package memory

import (
	"fmt"
	"testing"
)

func TestSearch(t *testing.T) {
	s := New()
	cases := map[string]int{
		"月": 10, "花": 9, "风": 8, "春": 10, "山": 9,
		"水": 8, "云": 8, "雪": 6, "日": 6, "人": 6, "夜": 6,
	}
	for kw, want := range cases {
		results := s.Search(kw)
		got := len(results)
		if got < want {
			t.Errorf("检索「%s」得 %d 首，期望至少 %d", kw, got, want)
		}
		fmt.Println(results)
	}
}

func TestSearchEmpty(t *testing.T) {
	s := New()
	if len(s.Search("")) != 0 {
		t.Error("空关键字应返回 0 结果")
	}
}

func TestAll(t *testing.T) {
	s := New()
	if len(s.All()) == 0 {
		t.Error("All 应返回非空诗词库")
	}
}
