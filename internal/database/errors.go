package database

import "errors"

// ErrNotFound 是 repository 层的「记录不存在」哨兵错误。
// 调用方应用 errors.Is(err, database.ErrNotFound) 判断。
var ErrNotFound = errors.New("database: record not found")

// ErrConflict 表示 UNIQUE 约束冲突或其它无法通过重试解决的写入冲突。
// 调用方负责把它翻译成业务语义（如 PlaylistService 把它包装成 ErrPlaylistNameConflict）。
var ErrConflict = errors.New("database: conflict")
