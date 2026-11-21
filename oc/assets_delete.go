package oc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Serialize Read/Delete/ExportPSD so an in-process deletion cannot race an export.
var assetOperations sync.Mutex

func regularAssetFile(path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("asset file is not a regular file: %s", filepath.Base(path))
	}
	return nil
}

// Read returns persisted metadata only when both associated files exist.
func (s *Store) Read(id string) (Asset, error) {
	assetOperations.Lock()
	defer assetOperations.Unlock()
	return s.readAsset(id)
}

func (s *Store) readAsset(id string) (Asset, error) {
	p, err := s.Path(id)
	if err != nil {
		return Asset{}, err
	}
	if err = regularAssetFile(p); err != nil {
		return Asset{}, err
	}
	meta := filepath.Join(s.Root, id+".json")
	if err = regularAssetFile(meta); err != nil {
		return Asset{}, err
	}
	f, err := os.Open(meta)
	if err != nil {
		return Asset{}, err
	}
	defer f.Close()
	b, err := readBounded(f, 1<<20)
	if err != nil {
		return Asset{}, err
	}
	var a Asset
	if err = json.Unmarshal(b, &a); err != nil {
		return Asset{}, err
	}
	if a.ID != id {
		return Asset{}, errors.New("asset metadata ID mismatch")
	}
	return a, nil
}

// Delete removes exactly id.png and id.json, never related parts or manifests.
// Metadata is hidden before removing the image, so List never reports a
// successfully deleted asset. A failed image removal restores its metadata.
func (s *Store) Delete(id string) error {
	assetOperations.Lock()
	defer assetOperations.Unlock()
	p, err := s.Path(id)
	if err != nil {
		return err
	}
	meta := filepath.Join(s.Root, id+".json")
	for _, path := range []string{p, meta} {
		if err = regularAssetFile(path); err != nil {
			return err
		}
	}
	tomb := filepath.Join(s.Root, ".delete-"+ID())
	if err = os.Rename(meta, tomb); err != nil {
		return err
	}
	if err = os.Remove(p); err != nil {
		return errors.Join(err, os.Rename(tomb, meta))
	}
	return os.Remove(tomb)
}
