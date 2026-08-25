// Package store 定义诗词数据的存储抽象层。
//
// 通过 Store 接口隔离「数据从哪来」：当前提供内存实现（memory），
// 后续可平滑接入 SQLite / MySQL / Elasticsearch / 向量数据库等中间件，
// 上层（tools / agent）只依赖接口，不感知具体存储。
package store

// Poem 一首诗的数据结构。
type Poem struct {
	Text   string `json:"text"`   // 诗句，如 "床前明月光，疑是地上霜"
	Author string `json:"author"` // 作者
	Title  string `json:"title"`  // 诗名
}

// Store 诗词数据访问接口。所有存储实现（内存 / SQLite / MySQL / ES / 向量库）均需实现。
type Store interface {
	// Search 按关键字（令字/主题词/作者）检索诗句。
	Search(keyword string) []Poem
	// All 返回全部诗词。
	All() []Poem
	// Close 释放底层资源（内存实现可为空操作）。
	Close() error
}
