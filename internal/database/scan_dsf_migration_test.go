package database_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

func TestScanSupportDSFMigration(t *testing.T) {
	ctx := context.Background()
	migration, err := os.ReadFile("migrations/0039_scan_support_dsf.sql")
	if err != nil {
		t.Fatal(err)
	}
	upSQL, _, _ := strings.Cut(string(migration), "-- +goose Down")
	cases := []struct {
		name, before, after string
	}{
		{"custom config", `{"supported_formats":["flac","mp3"],"auto_scan":false,"scan_interval":17,"custom":"dsf"}`, `{"supported_formats":["flac","mp3","dsf"],"auto_scan":false,"scan_interval":17,"custom":"dsf"}`},
		{"empty array", `{"supported_formats":[]}`, `{"supported_formats":["dsf"]}`},
		{"already present", `{"supported_formats":["dsf","flac"]}`, `{"supported_formats":["dsf","flac"]}`},
		{"uppercase present", `{"supported_formats":["DSF","mp3"]}`, `{"supported_formats":["DSF","mp3"]}`},
		{"mixed case present", `{"supported_formats":["DsF"]}`, `{"supported_formats":["DsF"]}`},
		{"missing array", `{"auto_scan":false}`, `{"auto_scan":false}`},
		{"null array", `{"supported_formats":null}`, `{"supported_formats":null}`},
		{"wrong type", `{"supported_formats":"flac"}`, `{"supported_formats":"flac"}`},
		{"malformed JSON", `not json`, `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.OpenMemoryDB(t)
			provider, err := goose.NewProvider(goose.DialectSQLite3, db.DB(), os.DirFS("migrations"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.DownTo(ctx, 38); err != nil {
				t.Fatal(err)
			}
			repo := db.ConfigRepository()
			if err := repo.Set(ctx, &models.Config{Key: "scan_config", Value: tc.before}); err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Up(ctx); err != nil {
				t.Fatalf("upgrade from version 38: %v", err)
			}
			// 再执行 Up SQL，验证幂等性而非只验证 goose 跳过已执行版本。
			if _, err := db.DB().ExecContext(ctx, upSQL); err != nil {
				t.Fatalf("repeat migration: %v", err)
			}
			config, err := repo.Get(ctx, "scan_config")
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if json.Valid([]byte(tc.after)) {
				if err := json.Unmarshal([]byte(config.Value), &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(tc.after), &want); err != nil {
					t.Fatal(err)
				}
			} else {
				got, want = config.Value, tc.after
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("config = %s, want %s", config.Value, tc.after)
			}
			if _, err := provider.DownTo(ctx, 38); err != nil {
				t.Fatalf("rollback: %v", err)
			}
			if fields, ok := want.(map[string]any); ok {
				if formats, ok := fields["supported_formats"].([]any); ok {
					remaining := make([]any, 0, len(formats))
					for _, format := range formats {
						if name, ok := format.(string); !ok || !strings.EqualFold(name, "dsf") {
							remaining = append(remaining, format)
						}
					}
					fields["supported_formats"] = remaining
				}
			}
			config, err = repo.Get(ctx, "scan_config")
			if err != nil {
				t.Fatal(err)
			}
			if json.Valid([]byte(config.Value)) {
				// 清空上次解析的 map，避免缺失字段被 json.Unmarshal 保留。
				got = nil
				if err := json.Unmarshal([]byte(config.Value), &got); err != nil {
					t.Fatal(err)
				}
			} else {
				got = config.Value
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rollback config = %s, want %#v", config.Value, want)
			}
			if _, err := provider.Up(ctx); err != nil {
				t.Fatalf("reapply after rollback: %v", err)
			}
		})
	}
}
