package models

import (
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users"`
	CreatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	ID            int       `bun:",pk,autoincrement"`
	FullName      string    `bun:"fullname,notnull"`
	Email         string    `bun:"email,notnull"`
}
