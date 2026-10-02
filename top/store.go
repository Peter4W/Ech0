// SPDX-License-Identifier: AGPL-3.0-or-later
// top 模块：置顶动态的持久化存储（pins.json）

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// PinStore 以有序 ID 列表形式保存置顶动态，服务端持久化，所有访客共享。
type PinStore struct {
	mu       sync.RWMutex
	filePath string
	ids      []string
}

func NewPinStore(dataDir string) (*PinStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	s := &PinStore{
		filePath: filepath.Join(dataDir, "pins.json"),
		ids:      []string{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PinStore) load() error {
	raw, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var ids []string
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &ids); err != nil {
			return err
		}
	}
	s.ids = dedupe(ids)
	return nil
}

// persist 调用方需持有写锁。先写临时文件再原子替换。
func (s *PinStore) persist() error {
	raw, err := json.MarshalIndent(s.ids, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

func (s *PinStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.ids))
	copy(out, s.ids)
	return out
}

func (s *PinStore) Has(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return indexOf(s.ids, id) >= 0
}

// Add 将动态置顶；新置顶排在最前。
func (s *PinStore) Add(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if indexOf(s.ids, id) >= 0 {
		return nil
	}
	s.ids = append([]string{id}, s.ids...)
	return s.persist()
}

func (s *PinStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := indexOf(s.ids, id)
	if idx < 0 {
		return nil
	}
	s.ids = append(s.ids[:idx], s.ids[idx+1:]...)
	return s.persist()
}

// Reorder 用传入的完整顺序覆盖；未包含的既有置顶会被丢弃，新出现的 ID 不允许加入。
func (s *PinStore) Reorder(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	want := dedupe(ids)
	allowed := make(map[string]bool, len(s.ids))
	for _, id := range s.ids {
		allowed[id] = true
	}
	for _, id := range want {
		if !allowed[id] {
			return errors.New("reorder contains unknown pin id: " + id)
		}
	}
	s.ids = want
	return s.persist()
}

// Prune 删除不在 valid 集合中的置顶 ID（用于清理已被删除/不可见的僵尸记录）。
func (s *PinStore) Prune(valid map[string]bool) (changed bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.ids[:0]
	for _, id := range s.ids {
		if valid[id] {
			kept = append(kept, id)
		} else {
			changed = true
		}
	}
	s.ids = kept
	if changed {
		err = s.persist()
	}
	return
}

func indexOf(ids []string, id string) int {
	for i, v := range ids {
		if v == id {
			return i
		}
	}
	return -1
}

func dedupe(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
