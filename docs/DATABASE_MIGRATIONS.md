# 数据库操作指南

涵盖三件事:**加 schema 变更**、**加/改 SQL 查询**、**写 Repository 与事务**。架构层面的方针(为什么这么分层)请看 [ARCHITECTURE_BACKEND.md](ARCHITECTURE_BACKEND.md);本文只讲怎么动手。

## 工具职责

| 工具 | 干什么 |
|---|---|
| **goose** | schema 版本管理:`migrations/000N_xxx.sql` 自动 Up |
| **sqlc** | 固定 SQL → 类型安全 Go 代码(`queries/*.sql` → `sqlc/*.sql.go`) |
| **squirrel** | 动态 SQL(变长 WHERE/SET/ORDER/分页),在 Repository 内现拼现执 |

入口在 `internal/database/sqlite.go` 的 `Open()`:启动时自动 `goose.Up` 到最新版本,无须手工触发。

## 新增 schema 变更(加表 / 加字段 / 加索引)

### 1. 新建迁移文件

文件名严格 `000N_xxx.sql`,编号递增,放在 `internal/database/migrations/`:

```bash
ls internal/database/migrations/
# 0001_init.sql
# 现在加一个:
touch internal/database/migrations/0002_add_song_lyric_offset.sql
```

### 2. 写 Up / Down 段

goose 用 `-- +goose Up` / `-- +goose Down` 分块,**多行语句必须**用 `StatementBegin/End` 包住(否则 goose 按分号切句会切坏 trigger / CREATE TABLE):

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

参考 `0001_init.sql` 看 trigger 和复合 CREATE TABLE 的写法。

### 3. 落库

什么都不用做。下次 `make run` / `make test` 启动时 goose 自动 Up,`schema_migrations` 表记录版本。

### 4. 同步 sqlc 模型

如果新字段需要被 sqlc 生成的查询读写,改完 `queries/*.sql` 后跑:

```bash
make sqlc
```

(详见下一节)

## 新增/修改 SQL 查询

### 1. 编辑对应表的 queries 文件

每张表一个文件:`internal/database/queries/{table}.sql`。加一段查询:

```sql
-- name: ListRecentSongs :many
SELECT id, title, artist FROM songs
ORDER BY added_at DESC LIMIT ?;
```

返回类型注释:

| 后缀 | 含义 | Go 签名 |
|---|---|---|
| `:one` | 单行,找不到返回 `sql.ErrNoRows` | `func(...) (Row, error)` |
| `:many` | 多行 | `func(...) ([]Row, error)` |
| `:exec` | 不关心结果 | `func(...) error` |
| `:execrows` | 影响行数 | `func(...) (int64, error)` |
| `:execlastid` | INSERT 拿自增 id | `func(...) (int64, error)` |

### 2. 生成代码

```bash
make sqlc
```

第一次跑会提示装 CLI(`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`),按提示装一次即可。

### 3. 入库

生成产物 `internal/database/sqlc/*.sql.go` **必须**提交:

```bash
git add internal/database/queries/songs.sql internal/database/sqlc/
```

运行时不依赖 sqlc CLI,只有改查询时才需要。

### 4. Repository 包装

sqlc 返回的是 `sqlc.Song`(对应表结构),通常需要在 Repository 转成 `models.Song`(领域模型),并把 `sql.ErrNoRows` 包装成 `database.ErrNotFound`:

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

## 动态 SQL(变长 WHERE / SET / 分页)

**不要**塞进 `queries/*.sql` —— sqlc 处理不了变长。直接在 `*_repository.go` 用 squirrel:

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

- **必须**用白名单校验 `OrderBy`(`filters.go` 已有 `songOrderWhitelist` 等);不在白名单的字段静默退化到默认排序,防 SQL 注入
- `applyOrder` / `applyPagination` 在 `filters.go` 共用,不要在 Repository 内重复实现
- squirrel 不接管 Scan,自己写 scan helper 或复用现成的

## 跨表事务

> 仅当一次写涉及**多张表**(典型例子:转换网络歌为本地歌需 `songs.INSERT` + `playlist_songs.UPDATE`)才需要事务。单表写不要包 RunInTx,徒增噪音。

### 用 UnitOfWork

```go
err := db.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
    if err := uow.Songs.Create(ctx, newSong); err != nil {
        return err
    }
    return uow.PlaylistSongs.ReplaceSong(ctx, playlistID, oldID, newSong.ID)
})
```

- `RunInTx` 自动 commit / rollback,`fn` 返回 err 则回滚
- `uow.Songs` / `uow.Playlists` / `uow.PlaylistSongs` 是**字段**(不是方法),指向绑定到当前 `*sql.Tx` 的 Repository 实例,共享同一连接,不会触发 `SQLITE_BUSY`
- service 层注入 `database.DB` 接口(`Close / RunInTx / 各 Repository getter`),需要事务时调用 `RunInTx`,需要无事务读写时直接 `db.SongRepository()` 等。参考 `internal/services/convert_service.go`

### 不要这么干

```go
// 反例 1:service 层手 Begin/Commit —— 没人维护,容易漏 rollback
tx, _ := db.BeginTx(ctx)
defer tx.Rollback()
...

// 反例 2:不同 Repository 实例传同一个 ctx 期望"自动事务" —— 没有这种事
r1.Create(ctx, ...)  // 这是两个独立连接
r2.Update(ctx, ...)  // 中间挂了就脏数据
```

## 测试

新写测试用 `internal/database/testutil.OpenMemoryDB(t)` 起 `:memory:` SQLite,跑真实迁移和真实 Repository。**禁止**手写 mockDB —— 之前已经全部删掉了,见 commit `a37070bd`。

```go
func TestSongService_Create(t *testing.T) {
    mdb := testutil.OpenMemoryDB(t)
    svc := services.NewSongService(mdb.SongRepository(), ...)

    got, err := svc.Create(ctx, &models.Song{Title: "test"})
    if err != nil { t.Fatal(err) }
    // ...真实断言,不是断言"mock 被调了几次"
}
```

注意:迁移会预置 id=1/2 两个内置歌单和若干 default config。写"统计某表行数"类断言时记得扣掉初始值。

## 错误语义

- 仓储未命中统一返回 `database.ErrNotFound`,service 层用 `errors.Is(err, database.ErrNotFound)` 判别
- UNIQUE 冲突由 Repository 翻译成 `database.ErrConflict`,service 层再包装成业务语义错误(如 `models.ErrPlaylistNameConflict`)
- 不要在 service 层直接判 `sql.ErrNoRows` —— 那是漏抽象

## 本地回滚(调试用)

启动时只有 `goose.Up`,**没有**自动 Down。本地想回滚一版:

```bash
# 装 goose CLI(一次)
go install github.com/pressly/goose/v3/cmd/goose@latest

# 回滚一版
cd internal/database/migrations
goose sqlite3 ../../../data/mimusic.db down
```

线上不要回滚。项目规约是"加字段往前走、不删字段",真要重置 `rm -f data/mimusic.db` 重新跑迁移更省事。

## 文件清单速查

```
internal/database/
├── migrations/         # goose 迁移源(0001_init.sql, 000N_xxx.sql)
├── queries/            # sqlc 输入(每表一个 *.sql)
├── sqlc/               # sqlc 输出(*.sql.go, 由 make sqlc 生成,入库)
├── testutil/memdb.go   # :memory: DB 工厂
├── *_repository.go     # 每表一个 Repository
├── unit_of_work.go     # 事务作用域内的 Repository 集合
├── filters.go          # Filter 类型 + 排序白名单 + applyOrder/applyPagination
├── errors.go           # ErrNotFound / ErrConflict 哨兵
└── sqlite.go           # Open() + RunInTx + Repository getter
```
