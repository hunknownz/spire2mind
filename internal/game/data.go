package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type DataStore struct {
	root  string
	cache sync.Map
}

func NewDataStore(root string) *DataStore {
	return &DataStore{root: root}
}

func (d *DataStore) Query(collection string, itemID string) (map[string]any, error) {
	data, err := d.loadCollection(collection)
	if err != nil {
		return nil, err
	}

	if itemID == "" {
		return map[string]any{
			"collection": collection,
			"count":      len(data),
		}, nil
	}

	value, ok := data[itemID]
	if !ok {
		return nil, fmt.Errorf("item %q not found in %s", itemID, collection)
	}
	return value, nil
}

func (d *DataStore) loadCollection(collection string) (map[string]map[string]any, error) {
	if cached, ok := d.cache.Load(collection); ok {
		return cached.(map[string]map[string]any), nil
	}

	path := filepath.Join(d.root, collection+".json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	raw := make(map[string]map[string]any)
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	d.cache.Store(collection, raw)
	return raw, nil
}
