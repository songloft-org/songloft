# Database Operations Guide

Covers three things: **adding schema changes**, **adding/modifying SQL queries**, and **writing Repositories and transactions**. For the architectural rationale (why the layering is the way it is), see [architecture_backend.md](architecture_backend.md); this document only covers the hands-on work.

## Tool Responsibilities

| Tool | What it does |
|---|---|
| **goose** | Schema version management: `migrations/000N_xxx.sql` auto-Up |
| **sqlc** | Fixed SQL → type-safe Go code (`queries/*.sql` → `sqlc/*.sql.go`) |
| **squirrel** | Dynamic SQL (variable-length WHERE/SET/ORDER/pagination), assembled and executed on the fly inside the Repository |

The entry point is `Open()` in `internal/database/sqlite.go`: on startup it automatically runs `goose.Up` to the latest version, no manual trigger needed.

Android x86_64 Bundle uses a cgo SQLite driver built with the Android NDK. The app sandbox on that architecture rejects the legacy Linux `lstat` syscall used by the pure-Go driver. Both drivers use the same database files, migrations, per-connection PRAGMAs, sort functions and repositories. Other builds continue to use modernc.org/sqlite.

## Adding a Schema Change (add table / add column / add index)

### 1. Create a new migration file

The filename must strictly follow `000N_xxx.sql`, with an incrementing number, placed in `internal/database/migrations/`:

```bash
ls internal/database/migrations/
# 0001_init.sql
# Now add one:
touch internal/database/migrations/0002_add_song_lyric_offset.sql
```

### 2. Write the Up / Down sections

goose splits blocks with `-- +goose Up` / `-- +goose Down`, and **multi-line statements must** be wrapped with `StatementBegin/End` (otherwise goose splits by semicolon and breaks triggers / CREATE TABLE):

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE songs ADD COLUMN lyric_offset INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE songs DROP COLUMN lyric_offset;
-- +goose StatementEnd
```

Refer to `0001_init.sql` to see how triggers and compound CREATE TABLE statements are written.

### 3. Persist it

Nothing to do. The next time you start with `make run` / `make test`, goose auto-runs Up and the `schema_migrations` table records the version.

### 4. Sync the sqlc models

If the new column needs to be read/written by sqlc-generated queries, after editing `queries/*.sql` run:

```bash
make sqlc
```

(See the next section for details)

## Adding/Modifying SQL Queries

### 1. Edit the queries file for the corresponding table

One file per table: `internal/database/queries/{table}.sql`. Add a query:

```sql
-- name: ListRecentSongs :many
SELECT id, title, artist FROM songs
ORDER BY added_at DESC LIMIT ?;
```

Return type annotations:

| Suffix | Meaning | Go signature |
|---|---|---|
| `:one` | Single row; returns `sql.ErrNoRows` if not found | `func(...) (Row, error)` |
| `:many` | Multiple rows | `func(...) ([]Row, error)` |
| `:exec` | Result not needed | `func(...) error` |
| `:execrows` | Affected row count | `func(...) (int64, error)` |
| `:execlastid` | INSERT returning the auto-increment id | `func(...) (int64, error)` |

### 2. Generate the code

```bash
make sqlc
```

The first run will prompt you to install the CLI (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`); just install it once as prompted.

### 3. Persist it

The generated artifacts `internal/database/sqlc/*.sql.go` **must** be committed:

```bash
git add internal/database/queries/songs.sql internal/database/sqlc/
```

Runtime does not depend on the sqlc CLI; you only need it when modifying queries.

### 4. Repository wrapping

sqlc returns `sqlc.Song` (matching the table structure), which usually needs to be converted in the Repository to `models.Song` (the domain model), and `sql.ErrNoRows` should be wrapped into `database.ErrNotFound`:

```go
func (r *SongRepository) ListRecent(ctx context.Context, limit int) ([]*models.Song, error) {
    rows, err := r.q.ListRecentSongs(ctx, int64(limit))
    if err != nil {
        return nil, err
    }
    out := make([]*models.Song, len(rows))
    for i, row := range rows {
        out[i] = sqlcSongToModel(row)
    }
    return out, nil
}
```

<a id="sqlc-common-errors"></a>

## Common sqlc Errors

- **Never write CJK comments after `-- name:`**: sqlc embeds query comments into generated code as doc comments **byte-wise**; multi-byte characters get truncated into invalid UTF-8 and `make sqlc` fails outright (`error generating code: illegal UTF-8 encoding`). Put such comments on the Go repository method instead
- **Qualify outer columns in self-referencing subqueries**: in `DELETE FROM t WHERE col = ? AND id NOT IN (SELECT x.id FROM t x ...)`, a bare outer `col` is rejected by sqlc as `column reference is ambiguous`; write `t.col`

## Dynamic SQL (variable-length WHERE / SET / pagination)

**Do not** stuff these into `queries/*.sql` — sqlc can't handle variable-length. Use squirrel directly in `*_repository.go`:

```go
sb := sq.Select("id", "title", "artist", "added_at").
    From("songs").
    PlaceholderFormat(sq.Question)

if f.Type != "" {
    sb = sb.Where(sq.Eq{"type": f.Type})
}
if f.Keyword != "" {
    like := "%" + f.Keyword + "%"
    sb = sb.Where(sq.Or{
        sq.Like{"title": like},
        sq.Like{"artist": like},
    })
}

sb = applyOrder(sb, f.OrderBy, f.Order, "added_at DESC", songOrderWhitelist, "")
sb = applyPagination(sb, f.Limit, f.Offset)

query, args, err := sb.ToSql()
if err != nil { return nil, err }

rows, err := r.db.QueryContext(ctx, query, args...)
// ...scan into []*models.Song
```

- You **must** validate `OrderBy` against a whitelist (`filters.go` already has `songOrderWhitelist` etc.); fields not in the whitelist silently fall back to the default ordering, preventing SQL injection
- `applyOrder` / `applyPagination` are shared in `filters.go`; don't reimplement them inside the Repository
- squirrel does not take over Scan; write your own scan helper or reuse an existing one

## Cross-Table Transactions

> Only when a single write touches **multiple tables** (typical example: converting a network song to a local song requires `songs.INSERT` + `playlist_songs.UPDATE`) do you need a transaction. Don't wrap single-table writes in RunInTx — it just adds noise.

### Use UnitOfWork

```go
err := db.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
    if err := uow.Songs.Create(ctx, newSong); err != nil {
        return err
    }
    return uow.PlaylistSongs.ReplaceSong(ctx, playlistID, oldID, newSong.ID)
})
```

- `RunInTx` automatically commits / rolls back; if `fn` returns an err, it rolls back
- `uow.Songs` / `uow.Playlists` / `uow.PlaylistSongs` / `uow.PlayHistory` / `uow.SongArtists` are **fields** (not methods), pointing to Repository instances bound to the current `*sql.Tx`, sharing the same connection, and won't trigger `SQLITE_BUSY`
- The service layer injects the `database.DB` interface (`Close / RunInTx / individual Repository getters`); call `RunInTx` when you need a transaction, and directly call `db.SongRepository()` etc. for non-transactional reads/writes. See the `RunInTx` usage in `internal/services/song_service.go` and `internal/services/play_history_service.go`

### Don't Do This

```go
// Anti-pattern 1: manually Begin/Commit in the service layer — no one maintains it, easy to miss a rollback
tx, _ := db.BeginTx(ctx)
defer tx.Rollback()
...

// Anti-pattern 2: passing the same ctx to different Repository instances expecting an "automatic transaction" — no such thing exists
r1.Create(ctx, ...)  // these are two independent connections
r2.Update(ctx, ...)  // if it dies in between, you get dirty data
```

## Testing

Write new tests with `internal/database/testutil.OpenMemoryDB(t)` to spin up a `:memory:` SQLite, running real migrations and real Repositories. **Do not** hand-write a mockDB — they were all deleted previously, see commit `a37070bd`.

```go
func TestSongService_Create(t *testing.T) {
    mdb := testutil.OpenMemoryDB(t)
    svc := services.NewSongService(mdb.SongRepository(), ...)

    got, err := svc.Create(ctx, &models.Song{Title: "test"})
    if err != nil { t.Fatal(err) }
    // ...real assertions, not asserting "how many times the mock was called"
}
```

Note: migrations preset the two built-in playlists with id=1/2 and several default configs. When writing "count rows in some table" assertions, remember to subtract the initial values.

## Error Semantics

- Repositories uniformly return `database.ErrNotFound` on a miss; the service layer uses `errors.Is(err, database.ErrNotFound)` to distinguish
- UNIQUE conflicts are translated by the Repository into `database.ErrConflict`, which the service layer then wraps into a business-semantic error (e.g. `models.ErrPlaylistNameConflict`)
- Don't check `sql.ErrNoRows` directly in the service layer — that's a leaky abstraction

## Local Rollback (for debugging)

On startup there's only `goose.Up`, and **no** automatic Down. To roll back one version locally:

```bash
# Install the goose CLI (once)
go install github.com/pressly/goose/v3/cmd/goose@latest

# Roll back one version
cd internal/database/migrations
goose sqlite3 ../../../data/songloft.db down
```

Use forward migrations in production and check backup / program compatibility before restoration. To verify migrations from an empty database, use a temporary database path instead of deleting an existing user database.

## File Reference Quick-Lookup

```
internal/database/
├── migrations/         # goose migration sources (0001_init.sql, 000N_xxx.sql)
├── queries/            # sqlc input (one *.sql per table)
├── sqlc/               # sqlc output (*.sql.go, generated by make sqlc, committed)
├── testutil/memdb.go   # :memory: DB factory
├── *_repository.go     # one Repository per table
├── unit_of_work.go     # set of Repositories scoped within a transaction
├── filters.go          # Filter types + sort whitelist + applyOrder/applyPagination
├── errors.go           # ErrNotFound / ErrConflict sentinels
└── sqlite.go           # Open() + RunInTx + Repository getters
```
