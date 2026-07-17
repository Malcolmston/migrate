package migrate

// This file implements "memdb", a minimal but genuine database/sql/driver used
// to drive the test suite. It is compiled only for tests. Despite being a fake
// store it implements the real driver.Driver / Conn / Stmt / Tx / Rows / Result
// interfaces, so the Migrator exercises the actual database/sql code paths
// (connection pooling, transactions, prepared statements, row scanning).
//
// It understands just enough SQL for the migrator's own bookkeeping:
//
//	CREATE TABLE [IF NOT EXISTS] <t> ( ... )
//	INSERT INTO <t> (cols...) VALUES (?, ...)
//	DELETE FROM <t> WHERE <col> = ?
//	SELECT <cols|*> FROM <t> [ORDER BY <col> [ASC|DESC]]
//
// Any other statement (arbitrary DDL/DML from a migration body) is accepted and
// recorded in an ordered execution log so tests can assert what ran, and in what
// order, without needing a full SQL engine. Transactions stage their mutations
// and only apply them (and append to the log) on Commit; Rollback discards them.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() { sql.Register("memdb", memDriver{}) }

// ---------------------------------------------------------------------------
// Shared store, keyed by DSN so each test can use an isolated database.
// ---------------------------------------------------------------------------

type memTable struct {
	cols []string
	rows [][]driver.Value
}

type memStore struct {
	mu      sync.Mutex
	tables  map[string]*memTable
	execLog []string
}

func (s *memStore) clone() *memStore {
	ns := &memStore{tables: make(map[string]*memTable, len(s.tables))}
	for name, t := range s.tables {
		nt := &memTable{cols: append([]string(nil), t.cols...)}
		for _, r := range t.rows {
			nt.rows = append(nt.rows, append([]driver.Value(nil), r...))
		}
		ns.tables[name] = nt
	}
	return ns
}

var (
	memMu     sync.Mutex
	memStores = map[string]*memStore{}
)

func getStore(dsn string) *memStore {
	memMu.Lock()
	defer memMu.Unlock()
	s := memStores[dsn]
	if s == nil {
		s = &memStore{tables: map[string]*memTable{}}
		memStores[dsn] = s
	}
	return s
}

// memExecLog returns a copy of the ordered list of mutating statements that have
// committed against the given DSN. Read-only SELECTs are not recorded.
func memExecLog(dsn string) []string {
	s := getStore(dsn)
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.execLog...)
}

// resetMemDB discards all state for a DSN so tests start from a clean slate.
func resetMemDB(dsn string) {
	memMu.Lock()
	defer memMu.Unlock()
	delete(memStores, dsn)
}

// ---------------------------------------------------------------------------
// driver.Driver / Conn / Tx / Stmt / Rows / Result
// ---------------------------------------------------------------------------

type memDriver struct{}

func (memDriver) Open(dsn string) (driver.Conn, error) {
	return &memConn{store: getStore(dsn)}, nil
}

type memConn struct {
	store *memStore
	tx    *memTx
}

func (c *memConn) Prepare(query string) (driver.Stmt, error) {
	return &memStmt{conn: c, query: query}, nil
}

func (c *memConn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	return &memStmt{conn: c, query: query}, nil
}

func (c *memConn) Close() error { return nil }

func (c *memConn) Begin() (driver.Tx, error) { return c.begin() }

func (c *memConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	return c.begin()
}

func (c *memConn) begin() (driver.Tx, error) {
	if c.tx != nil {
		return nil, fmt.Errorf("memdb: nested transactions are not supported")
	}
	c.tx = &memTx{conn: c}
	return c.tx, nil
}

// memOp is a staged mutation. fn applies it to the store and reports whether it
// actually changed anything; only changing statements are recorded in the log.
type memOp struct {
	fn  func(*memStore) bool
	log string
}

type memTx struct {
	conn *memConn
	ops  []memOp
}

func (t *memTx) Commit() error {
	s := t.conn.store
	s.mu.Lock()
	for _, op := range t.ops {
		if op.fn(s) {
			s.execLog = append(s.execLog, op.log)
		}
	}
	s.mu.Unlock()
	t.conn.tx = nil
	return nil
}

func (t *memTx) Rollback() error {
	t.conn.tx = nil
	return nil
}

type memStmt struct {
	conn  *memConn
	query string
}

func (s *memStmt) Close() error  { return nil }
func (s *memStmt) NumInput() int { return -1 }

func (s *memStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.conn.exec(s.query, args)
}

func (s *memStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.conn.query(s.query, args)
}

type memResult struct{}

func (memResult) LastInsertId() (int64, error) { return 0, nil }
func (memResult) RowsAffected() (int64, error) { return 0, nil }

type memRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *memRows) Columns() []string { return r.cols }
func (r *memRows) Close() error      { return nil }

func (r *memRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

// ---------------------------------------------------------------------------
// Execution: mutating statements are planned into a store operation plus a log
// entry, then either staged in the transaction or applied immediately.
// ---------------------------------------------------------------------------

func (c *memConn) exec(query string, args []driver.Value) (driver.Result, error) {
	op, logEntry, err := planExec(query, args)
	if err != nil {
		return nil, err
	}
	if c.tx != nil {
		c.tx.ops = append(c.tx.ops, memOp{fn: op, log: logEntry})
	} else {
		s := c.store
		s.mu.Lock()
		if op(s) {
			s.execLog = append(s.execLog, logEntry)
		}
		s.mu.Unlock()
	}
	return memResult{}, nil
}

var (
	wsRe       = regexp.MustCompile(`\s+`)
	createRe   = regexp.MustCompile(`(?i)^CREATE\s+TABLE\s+(IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)$`)
	insertRe   = regexp.MustCompile(`(?i)^INSERT\s+INTO\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s+VALUES\s*\(([^)]*)\)$`)
	deleteRe   = regexp.MustCompile(`(?i)^DELETE\s+FROM\s+([A-Za-z_][A-Za-z0-9_]*)\s+WHERE\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$`)
	dropRe     = regexp.MustCompile(`(?i)^DROP\s+TABLE\s+(IF\s+EXISTS\s+)?([A-Za-z_][A-Za-z0-9_]*)$`)
	selectRe   = regexp.MustCompile(`(?i)^SELECT\s+(.+?)\s+FROM\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s+ORDER\s+BY\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s+(ASC|DESC))?)?$`)
	identColRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func normalizeSpace(s string) string {
	return strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
}

// planExec turns a mutating statement into a store operation and a log entry.
// The operation reports whether it actually changed the store, so no-op
// statements (such as CREATE TABLE IF NOT EXISTS on an existing table) are not
// recorded in the execution log.
func planExec(query string, args []driver.Value) (func(*memStore) bool, string, error) {
	q := normalizeSpace(query)

	if m := createRe.FindStringSubmatch(q); m != nil {
		ifNotExists := m[1] != ""
		name := m[2]
		cols := parseColumnNames(m[3])
		return func(s *memStore) bool {
			if _, ok := s.tables[name]; ok && ifNotExists {
				return false
			}
			s.tables[name] = &memTable{cols: cols}
			return true
		}, q, nil
	}

	if m := insertRe.FindStringSubmatch(q); m != nil {
		name := m[1]
		cols := splitList(m[2])
		values, err := parseValueList(m[3], args)
		if err != nil {
			return nil, "", err
		}
		if len(values) != len(cols) {
			return nil, "", fmt.Errorf("memdb: INSERT column/value count mismatch for %s", name)
		}
		return func(s *memStore) bool {
			t := s.tables[name]
			if t == nil {
				t = &memTable{cols: cols}
				s.tables[name] = t
			}
			row := make([]driver.Value, len(t.cols))
			for i, col := range t.cols {
				if idx := indexOf(cols, col); idx >= 0 {
					row[i] = values[idx]
				}
			}
			t.rows = append(t.rows, row)
			return true
		}, q, nil
	}

	if m := deleteRe.FindStringSubmatch(q); m != nil {
		name, col := m[1], m[2]
		vals, err := parseValueList(m[3], args)
		if err != nil {
			return nil, "", err
		}
		target := vals[0]
		return func(s *memStore) bool {
			t := s.tables[name]
			if t == nil {
				return false
			}
			ci := indexOf(t.cols, col)
			if ci < 0 {
				return false
			}
			kept := t.rows[:0:0]
			for _, r := range t.rows {
				if compareValues(r[ci], target) != 0 {
					kept = append(kept, r)
				}
			}
			removed := len(kept) != len(t.rows)
			t.rows = kept
			return removed
		}, q, nil
	}

	if m := dropRe.FindStringSubmatch(q); m != nil {
		name := m[2]
		return func(s *memStore) bool {
			if _, ok := s.tables[name]; !ok {
				return false
			}
			delete(s.tables, name)
			return true
		}, q, nil
	}

	// Arbitrary DDL/DML: accept and record, but do not model its effect.
	return func(*memStore) bool { return true }, q, nil
}

func (c *memConn) query(query string, args []driver.Value) (driver.Rows, error) {
	_ = args
	q := normalizeSpace(query)
	m := selectRe.FindStringSubmatch(q)
	if m == nil {
		return nil, fmt.Errorf("memdb: unsupported query: %s", q)
	}
	selCols := splitList(m[1])
	name := m[2]
	orderCol := m[3]
	desc := strings.EqualFold(m[4], "DESC")

	c.store.mu.Lock()
	defer c.store.mu.Unlock()

	view := c.store
	if c.tx != nil {
		view = c.store.clone()
		for _, op := range c.tx.ops {
			op.fn(view)
		}
	}

	t := view.tables[name]
	if t == nil {
		return nil, fmt.Errorf("memdb: no such table: %s", name)
	}

	outCols := selCols
	if len(selCols) == 1 && selCols[0] == "*" {
		outCols = append([]string(nil), t.cols...)
	}
	colIdx := make([]int, len(outCols))
	for i, col := range outCols {
		ci := indexOf(t.cols, col)
		if ci < 0 {
			return nil, fmt.Errorf("memdb: no such column %q in %s", col, name)
		}
		colIdx[i] = ci
	}

	src := append([][]driver.Value(nil), t.rows...)
	if orderCol != "" {
		oc := indexOf(t.cols, orderCol)
		if oc < 0 {
			return nil, fmt.Errorf("memdb: no such order column %q in %s", orderCol, name)
		}
		sort.SliceStable(src, func(i, j int) bool {
			cmp := compareValues(src[i][oc], src[j][oc])
			if desc {
				return cmp > 0
			}
			return cmp < 0
		})
	}

	rows := make([][]driver.Value, len(src))
	for i, r := range src {
		out := make([]driver.Value, len(colIdx))
		for j, ci := range colIdx {
			out[j] = r[ci]
		}
		rows[i] = out
	}
	return &memRows{cols: outCols, rows: rows}, nil
}

// ---------------------------------------------------------------------------
// Tiny parsing/value helpers.
// ---------------------------------------------------------------------------

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// parseColumnNames extracts column names from a CREATE TABLE body, skipping
// table-level constraint clauses.
func parseColumnNames(body string) []string {
	var cols []string
	for _, part := range splitTopLevel(body) {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "PRIMARY", "FOREIGN", "UNIQUE", "CONSTRAINT", "CHECK":
			continue
		}
		if identColRe.MatchString(fields[0]) {
			cols = append(cols, fields[0])
		}
	}
	return cols
}

// splitTopLevel splits on commas that are not nested inside parentheses.
func splitTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(s[start:]); last != "" {
		out = append(out, last)
	}
	return out
}

// parseValueList resolves a VALUES/WHERE value list, substituting '?' with the
// next positional argument and parsing simple integer / quoted-string literals.
func parseValueList(s string, args []driver.Value) ([]driver.Value, error) {
	tokens := splitList(s)
	out := make([]driver.Value, 0, len(tokens))
	argi := 0
	for _, tok := range tokens {
		switch {
		case tok == "?":
			if argi >= len(args) {
				return nil, fmt.Errorf("memdb: not enough arguments")
			}
			out = append(out, args[argi])
			argi++
		case strings.HasPrefix(tok, "'") && strings.HasSuffix(tok, "'"):
			out = append(out, strings.ReplaceAll(tok[1:len(tok)-1], "''", "'"))
		default:
			if n, err := strconv.ParseInt(tok, 10, 64); err == nil {
				out = append(out, n)
			} else {
				out = append(out, tok)
			}
		}
	}
	return out, nil
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

// compareValues orders the driver.Value kinds the store actually holds: signed
// integers, strings, and timestamps. Mixed/unknown kinds compare as equal.
func compareValues(a, b driver.Value) int {
	switch av := a.(type) {
	case int64:
		if bv, ok := b.(int64); ok {
			switch {
			case av < bv:
				return -1
			case av > bv:
				return 1
			default:
				return 0
			}
		}
	case string:
		if bv, ok := b.(string); ok {
			return strings.Compare(av, bv)
		}
	case time.Time:
		if bv, ok := b.(time.Time); ok {
			return av.Compare(bv)
		}
	}
	return 0
}
