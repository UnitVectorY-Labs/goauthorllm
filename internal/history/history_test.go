package history

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHistoryPersistsPerDocumentAndKind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.md")
	if entries, err := Load(path, "directed"); err != nil || len(entries) != 0 {
		t.Fatalf("initial load: %v %v", entries, err)
	}
	if _, err := os.Stat(Path(path)); !os.IsNotExist(err) {
		t.Fatal("reading should not create a database")
	}
	for _, value := range []string{"first\nline", "first\nline", "second", "first\nline"} {
		if err := Save(path, "directed", value); err != nil {
			t.Fatal(err)
		}
	}
	if err := Save(path, "generation", "guidance"); err != nil {
		t.Fatal(err)
	}
	entries, err := Load(path, "directed")
	if err != nil || !reflect.DeepEqual(entries, []string{"first\nline", "second", "first\nline"}) {
		t.Fatalf("history: %v %v", entries, err)
	}
	other, err := Load(path+"other", "directed")
	if err != nil || len(other) != 0 {
		t.Fatalf("other file: %v %v", other, err)
	}
}
