package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Whoisvox/godev/shortlink/internal/codegen"
	"github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	Db *sql.DB
}

func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Second)
	return &MySQLStore{Db: db}, nil
}

func (s *MySQLStore) Ping() error {
	return s.Db.Ping()
}

// Create 生成短码 -> 插入 -> 1062 冲突重试
func (s *MySQLStore) Create(ctx context.Context, url string) (string, error) {
	const maxRetries = 3

	for range maxRetries {
		code, err := codegen.New(6)
		if err != nil {
			return "", nil
		}

		_, err = s.Db.ExecContext(
			ctx,
			"INSERT INTO links (code, url) VALUES (?, ?)",
			code, url)
		if err == nil {
			return code, nil // 成功
		}

		// 判断是不是1062重复健
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			// 冲突了， 重试
			continue
		}
		// 其他错误直接返回
		return "", err
	}

	return "", errors.New("failed to generate unique code after retries")
}

func (s *MySQLStore) Get(ctx context.Context, code string) (string, error) {
	var url string
	err := s.Db.QueryRowContext(ctx,
		"SELECT url FROM links WHERE code = ?", code,
	).Scan(&url)

	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}

	if err != nil {
		return "", err
	}

	return url, nil
}
