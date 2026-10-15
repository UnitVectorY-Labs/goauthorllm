// Package history stores instructions in a document's SQLite sidecar.
package history

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Path(document string) string { return document + ".goauthorllm.sqlite" }

func open(document string) (*sql.DB, error) {
	path := Path(document)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	file.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS instructions (id INTEGER PRIMARY KEY, kind TEXT NOT NULL, text TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Save(document, kind, text string) error {
	if document == "" || strings.TrimSpace(text) == "" {
		return nil
	}
	db, err := open(document)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO instructions(kind,text) SELECT ?,? WHERE COALESCE((SELECT text FROM instructions WHERE kind=? ORDER BY id DESC LIMIT 1),'') <> ?`, kind, text, kind, text)
	return err
}

// Load returns newest first without creating a sidecar just by opening a document.
func Load(document, kind string) ([]string, error) {
	if _, err := os.Stat(Path(document)); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	db, err := open(document)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT text FROM instructions WHERE kind=? ORDER BY id DESC`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		entries = append(entries, value)
	}
	return entries, rows.Err()
}
