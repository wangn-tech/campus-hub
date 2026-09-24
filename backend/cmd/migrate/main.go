package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/wangn-tech/campus-hub/internal/config"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down")
	flag.Parse()
	cfg, err := config.Load("configs/config.dev.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?multiStatements=true&parseTime=true", cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
	m, err := migrate.New("file://migrations/mysql", "mysql://"+dsn)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	defer m.Close()
	switch *direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		log.Fatalf("unsupported direction %q", *direction)
	}
	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("run migration: %v", err)
	}
}
