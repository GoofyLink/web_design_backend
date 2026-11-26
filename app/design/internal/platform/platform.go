package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Error struct {
	Status  int
	Code    int
	Message string
}

func (e *Error) Error() string              { return e.Message }
func Fail(status int, message string) error { return &Error{status, status * 100, message} }
func Normalize(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return Fail(404, "资源不存在")
	}
	var m *mysql.MySQLError
	if errors.As(err, &m) && m.Number == 1062 {
		return Fail(409, "记录已存在")
	}
	return err
}
func Random() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func ID(n int64) string { return strconv.FormatInt(n, 10) }
func ParseID(s string) (int64, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n < 1 {
		return 0, Fail(400, "ID 格式错误")
	}
	return n, nil
}
func Tx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	t, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer t.Rollback()
	if e = fn(t); e != nil {
		return Normalize(e)
	}
	return Normalize(t.Commit())
}
func Audit(ctx context.Context, t *sql.Tx, actor int64, action, target, requestID string) error {
	_, e := t.ExecContext(ctx, `INSERT INTO design_audit_logs(actor_id,action,target,request_id,created_at) VALUES(?,?,?,?,?)`, actor, action, target, requestID, time.Now().UTC())
	return e
}
func Affected(r sql.Result, e error) error {
	if e != nil {
		return Normalize(e)
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return Fail(404, "资源不存在")
	}
	return nil
}
func Open(dsn string) (*sql.DB, error) {
	c, e := mysql.ParseDSN(dsn)
	if e != nil {
		return nil, fmt.Errorf("invalid MySQL DSN")
	}
	if c.DBName == "" {
		return nil, fmt.Errorf("MySQL DSN must select a database")
	}
	c.ParseTime = true
	c.Loc = time.UTC
	c.MultiStatements = false
	c.ClientFoundRows = true
	c.Timeout = 5 * time.Second
	c.ReadTimeout = 10 * time.Second
	c.WriteTimeout = 10 * time.Second
	if c.Params == nil {
		c.Params = map[string]string{}
	}
	c.Params["time_zone"] = "'+00:00'"
	db, e := sql.Open("mysql", c.FormatDSN())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, fmt.Errorf("MySQL connection failed: %w", e)
	}
	return db, nil
}

type Page struct {
	Number  int
	Size    int
	Keyword string
	Status  string
}
type List struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func (p Page) Offset() int { return (p.Number - 1) * p.Size }
