package main

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	"testing"
)

func TestBuildMySQLMigrationURLUsesEscapedConfig(t *testing.T) {
	cfg := ConfigPackage.HGMySQLConfig{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "root",
		Password: "p@ss:word",
		Database: "HG_MLC_DB",
	}

	got := buildMySQLMigrationURL(cfg)
	want := "mysql://root:p%40ss%3Aword@tcp(127.0.0.1:3306)/HG_MLC_DB"
	if got != want {
		t.Fatalf("migration URL mismatch: got %q want %q", got, want)
	}
}
