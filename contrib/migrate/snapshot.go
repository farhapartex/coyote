package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/farhapartex/coyote/core/model"
)

const SnapshotFile = "snapshot.json"

type Snapshot struct {
	Version int     `json:"version"`
	Tables  []Table `json:"tables"`
}

func (s Snapshot) Table(name string) (Table, bool) {
	for _, t := range s.Tables {
		if t.Name == name {
			return t, true
		}
	}
	return Table{}, false
}

func SnapshotOf(schemas []*model.Schema) Snapshot {
	snapshot := Snapshot{Version: 1}
	for _, schema := range schemas {
		snapshot.Tables = append(snapshot.Tables, TableOf(schema))
	}
	sort.Slice(snapshot.Tables, func(i, j int) bool { return snapshot.Tables[i].Name < snapshot.Tables[j].Name })
	return snapshot
}

func LoadSnapshot(dir string) (Snapshot, error) {
	raw, err := os.ReadFile(filepath.Join(dir, SnapshotFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{Version: 1}, nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("coyote/migrate: reading %s: %w", SnapshotFile, err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("coyote/migrate: parsing %s: %w", SnapshotFile, err)
	}
	return snapshot, nil
}

func SaveSnapshot(dir string, snapshot Snapshot) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("coyote/migrate: creating %s: %w", dir, err)
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("coyote/migrate: encoding snapshot: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(dir, SnapshotFile), raw, 0o644); err != nil {
		return fmt.Errorf("coyote/migrate: writing %s: %w", SnapshotFile, err)
	}
	return nil
}
