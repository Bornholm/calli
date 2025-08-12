package sqlite

import (
	"fmt"
	"log"
	"time"

	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitemigration"
	"zombiezen.com/go/sqlite/sqlitex"
)

const Type filesystem.Type = "sqlite"

func init() {
	filesystem.Register(Type, CreateFileSystemFromOptions)
}

type Options struct {
	Path string `mapstructure:"path"`
}

func CreateFileSystemFromOptions(options any) (webdav.FileSystem, error) {
	opts := Options{}

	if err := mapstructure.Decode(options, &opts); err != nil {
		return nil, errors.Wrapf(err, "could not parse '%s' filesystem options", Type)
	}

	schema := sqlitemigration.Schema{
		Migrations: []string{
			`CREATE TABLE IF NOT EXISTS files (
					path TEXT PRIMARY KEY,     -- File path (used as unique identifier)
					is_dir INTEGER NOT NULL,   -- 1 if directory, 0 if file
					mode INTEGER NOT NULL,     -- File permissions
					size INTEGER NOT NULL,     -- File size in bytes (0 for directories)
					mtime INTEGER NOT NULL     -- Modification time (Unix timestamp)
				);
			`,
			`CREATE INDEX IF NOT EXISTS idx_parent_path ON files(path);`,
			`CREATE TABLE IF NOT EXISTS file_contents (
					path TEXT PRIMARY KEY REFERENCES files(path) ON DELETE CASCADE,
					content BLOB              -- File content
				);
			`,
		},
		RepeatableMigration: fmt.Sprintf(`INSERT OR IGNORE INTO files (path, is_dir, mode, size, mtime) VALUES ('/', 1, 493, 0, %d)`, time.Now().Unix()),
	}

	pool := sqlitemigration.NewPool(opts.Path, schema, sqlitemigration.Options{
		Flags: sqlite.OpenCreate | sqlite.OpenReadWrite | sqlite.OpenWAL,
		PrepareConn: func(conn *sqlite.Conn) error {
			return sqlitex.ExecScript(conn, `PRAGMA foreign_keys = ON; PRAGMA auto_vacuum=FULL`)
		},
		OnError: func(e error) {
			log.Println(e)
		},
	})

	fs := NewFileSystem(pool)

	return fs, nil
}
