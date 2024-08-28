package db

import (
	"context"
	"database/sql"
	"errors"

	"github.com/betterstack-community/go-image-upload/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bunotel"
)

type DBConn struct {
	db *bun.DB
}

func NewDBConn(ctx context.Context, name, url string) (*DBConn, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(url)))

	db := bun.NewDB(sqldb, pgdialect.New())
	db.AddQueryHook(
		bunotel.NewQueryHook(bunotel.WithDBName(name)),
	)
	return &DBConn{db}, nil
}

func (conn *DBConn) GetUser(ctx context.Context, user *models.User) error {
	return conn.db.NewSelect().
		Model(user).
		Where("email = ?", user.Email).
		Scan(ctx)
}

func (conn *DBConn) FindOrCreateUser(
	ctx context.Context,
	user *models.User,
) error {
	err := conn.GetUser(ctx, user)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = conn.db.NewInsert().Model(user).Exec(ctx)

	return err
}
