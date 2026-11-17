package database

import (
	"context"
	"testing"

	"songloft/internal/models"
)

// intPtr 返回 int 指针，构造 ExactRating 过滤用。
func intPtr(v int) *int { return &v }

// seedRatedSong 插入一首本地歌曲并设置评级。
func seedRatedSong(t *testing.T, repo *SongRepository, path string, rating int) *models.Song {
	t.Helper()
	song := &models.Song{
		Type:     models.TypeLocal,
		Title:    path,
		FilePath: path,
		Duration: 180,
	}
	ctx := context.Background()
	if err := repo.Create(ctx, song); err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	if rating > 0 {
		if _, err := repo.UpdateRatingBatch(ctx, []int64{song.ID}, rating); err != nil {
			t.Fatalf("update rating %s: %v", path, err)
		}
	}
	return song
}

// TestUpdateRating_ThenGetByID 单首评级写入后可读回。
func TestUpdateRating_ThenGetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := db.SongRepository()

	song := seedRatedSong(t, repo, "/m/a.mp3", 0)
	if song.Rating != 0 {
		t.Fatalf("new song rating: got %d want 0", song.Rating)
	}

	if err := repo.UpdateRating(context.Background(), song.ID, 80); err != nil {
		t.Fatalf("UpdateRating: %v", err)
	}
	got, err := repo.GetByID(context.Background(), song.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Rating != 80 {
		t.Fatalf("rating: got %d want 80", got.Rating)
	}

	// 清除评级回到 0
	if err := repo.UpdateRating(context.Background(), song.ID, 0); err != nil {
		t.Fatalf("UpdateRating clear: %v", err)
	}
	got, err = repo.GetByID(context.Background(), song.ID)
	if err != nil {
		t.Fatalf("GetByID after clear: %v", err)
	}
	if got.Rating != 0 {
		t.Fatalf("rating after clear: got %d want 0", got.Rating)
	}
}

// TestList_FilterByRating MinRating/ExactRating 过滤语义：
// MinRating 为「≥ 下限」；ExactRating 精确匹配（含 0=未评级）。
func TestList_FilterByRating(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := db.SongRepository()

	ctx := context.Background()
	seedRatedSong(t, repo, "/m/none.mp3", 0)
	seedRatedSong(t, repo, "/m/one.mp3", 20)
	seedRatedSong(t, repo, "/m/four.mp3", 80)
	seedRatedSong(t, repo, "/m/five.mp3", 100)

	cases := []struct {
		name    string
		filter  SongFilter
		wantIDs int
	}{
		{"min 20 = 至少1星", SongFilter{MinRating: 20}, 3},
		{"min 80 = 至少4星", SongFilter{MinRating: 80}, 2},
		{"exact 0 = 仅未评级", SongFilter{ExactRating: intPtr(0)}, 1},
		{"exact 100 = 仅5星", SongFilter{ExactRating: intPtr(100)}, 1},
		{"无过滤条件", SongFilter{}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.List(ctx, &tc.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(got) != tc.wantIDs {
				t.Fatalf("count: got %d want %d (%+v)", len(got), tc.wantIDs, got)
			}
		})
	}
}

// TestList_OrderByRating 按 rating 排序：ASC 时未评级在前（与 track 的 NULL 兜底语义一致靠 ASC 排前面由 SQL 值序决定）。
func TestList_OrderByRating(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := db.SongRepository()

	ctx := context.Background()
	seedRatedSong(t, repo, "/m/five.mp3", 100)
	seedRatedSong(t, repo, "/m/none.mp3", 0)
	seedRatedSong(t, repo, "/m/one.mp3", 20)

	asc, err := repo.List(ctx, &SongFilter{OrderBy: "rating", Order: "asc"})
	if err != nil {
		t.Fatalf("List asc: %v", err)
	}
	if len(asc) != 3 || asc[0].Rating != 0 || asc[2].Rating != 100 {
		t.Fatalf("asc order: got %v", asc)
	}
	desc, err := repo.List(ctx, &SongFilter{OrderBy: "rating", Order: "desc"})
	if err != nil {
		t.Fatalf("List desc: %v", err)
	}
	if len(desc) != 3 || desc[0].Rating != 100 || desc[2].Rating != 0 {
		t.Fatalf("desc order: got %v", desc)
	}
}

// TestListFacet_Rating rating 维度聚合：未评级不参与，取值为评级数值字符串。
func TestListFacet_Rating(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := db.SongRepository()

	ctx := context.Background()
	seedRatedSong(t, repo, "/m/a.mp3", 20)
	seedRatedSong(t, repo, "/m/b.mp3", 20)
	seedRatedSong(t, repo, "/m/c.mp3", 80)
	seedRatedSong(t, repo, "/m/unrated.mp3", 0)

	facets, err := repo.ListFacet(ctx, "rating", &FacetFilter{})
	if err != nil {
		t.Fatalf("ListFacet rating: %v", err)
	}
	if len(facets) != 2 {
		t.Fatalf("facet count: got %d want 2 (%+v)", len(facets), facets)
	}
	byValue := map[string]int64{}
	for _, f := range facets {
		byValue[f.Value] = f.Count
	}
	if byValue["20"] != 2 || byValue["80"] != 1 {
		t.Fatalf("facet values: %+v", byValue)
	}
}

// TestUpdateRatingBatch_Mixed 批量评级：部分 ID 不存在时静默跳过，返回实际更新数。
func TestUpdateRatingBatch_Mixed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := db.SongRepository()

	ctx := context.Background()
	s1 := seedRatedSong(t, repo, "/m/a.mp3", 0)
	s2 := seedRatedSong(t, repo, "/m/b.mp3", 0)

	updated, err := repo.UpdateRatingBatch(ctx, []int64{s1.ID, s2.ID, 99999}, 60)
	if err != nil {
		t.Fatalf("UpdateRatingBatch: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated: got %d want 2", updated)
	}
	got, _ := repo.GetByID(ctx, s1.ID)
	if got.Rating != 60 {
		t.Fatalf("s1 rating: got %d want 60", got.Rating)
	}
}

// TestUpdateRating_NotFound 更新不存在的歌曲返回 ErrNotFound。
func TestUpdateRating_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := db.SongRepository()

	err := repo.UpdateRating(context.Background(), 424242, 50)
	if err != ErrNotFound {
		t.Fatalf("err: got %v want ErrNotFound", err)
	}
}
