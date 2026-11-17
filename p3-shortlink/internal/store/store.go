package store

import (
	"context"
	"errors"
)

// 哨兵错误 -- 上层只依赖这个，不依赖sql.ErrNoRows
var ErrNotFound = errors.New("link not found")

type Store interface {
	Create(ctx context.Context, url string) (code string, err error)
	Get(ctx context.Context, code string) (url string, err error)
}
