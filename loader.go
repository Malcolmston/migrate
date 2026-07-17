package migrate

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// fileRe matches migration file names of the form
//
//	<version>_<name>.(up|down).sql
//
// where <version> is one or more decimal digits and <name> is any non-empty
// run of characters up to the direction suffix.
var fileRe = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

// LoadDir loads migrations from files in the given directory. It is a thin
// wrapper around [LoadFS] using os.DirFS.
func LoadDir(dir string) ([]Migration, error) {
	return LoadFS(os.DirFS(dir))
}

// LoadFS loads migrations from the root of fsys. Files must be named
// "<version>_<name>.up.sql" and "<version>_<name>.down.sql". The up file is
// required; the down file is optional (its absence yields an irreversible
// migration). Files that do not match the naming pattern are ignored.
//
// The returned slice is sorted ascending by version. An error is returned for
// duplicate versions, unreadable files, or an up/down pair whose names disagree.
func LoadFS(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	byVersion := make(map[uint64]*Migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := fileRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		version, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse version in %q: %w", entry.Name(), err)
		}
		if version == 0 {
			return nil, fmt.Errorf("%w: version must be non-zero in %q", ErrInvalidMigration, entry.Name())
		}
		name, direction := m[2], m[3]

		body, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", entry.Name(), err)
		}

		mig := byVersion[version]
		if mig == nil {
			mig = &Migration{Version: version, Name: name}
			byVersion[version] = mig
		} else if mig.Name != name {
			return nil, fmt.Errorf("version %d has mismatched names %q and %q", version, mig.Name, name)
		}

		switch direction {
		case "up":
			if mig.UpSQL != "" {
				return nil, fmt.Errorf("%w: %d", ErrDuplicateVersion, version)
			}
			mig.UpSQL = string(body)
		case "down":
			mig.DownSQL = string(body)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if strings.TrimSpace(m.UpSQL) == "" {
			return nil, fmt.Errorf("%w: version %d %q has no up file", ErrInvalidMigration, m.Version, m.Name)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
