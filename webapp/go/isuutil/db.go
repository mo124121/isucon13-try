package isuutil

import (
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
)

const (
	driverName = "mysql"
)

func NewIsuconDB(config *mysql.Config) (*sqlx.DB, error) {

	return newIsuconDB(config)
}

func newIsuconDB(config *mysql.Config) (*sqlx.DB, error) {
	// ISUCONにおける必須の設定項目たち
	config.ParseTime = true
	config.InterpolateParams = true

	// OpenTelemetryのSQLインストルメンテーションを有効にすることで、Jaegerから発行されているSQLを見れるようにしている
	stdDb, err := otelsql.Open(driverName, config.FormatDSN(), otelsql.WithDBName(driverName))
	if err != nil {
		return nil, fmt.Errorf("failed to open to DB: %w", err)
	}

	dbx := sqlx.NewDb(stdDb, driverName)

	// コネクション数はデフォルトでは無制限になっている。
	// 数十から数百くらいで要調整。
	dbx.SetMaxOpenConns(20)
	dbx.SetMaxIdleConns(20)
	dbx.SetConnMaxLifetime(5 * time.Minute)

	// 再起動試験対策
	// Pingして接続が確立するまで待つ
	for {
		if err := dbx.Ping(); err == nil {
			break
		} else {
			fmt.Println(err)
			time.Sleep(time.Second * 2)
		}
	}
	fmt.Println("ISUCON DB ready")

	return dbx, nil
}
