package database

import (
	"context"
	"fmt"
	"testing"

	"songloft/internal/models"
)

// seedFacetSongs 造几首带标签维度的本地歌曲，供 facet / 过滤测试复用。
func seedFacetSongs(t *testing.T, db DB) {
	t.Helper()
	ctx := context.Background()
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "A", Artist: "周杰伦", Album: "范特西", Genre: "Pop", Language: "国语", Style: "R&B", Year: 2001, FilePath: "/m/a.mp3"},
		{Type: models.TypeLocal, Title: "B", Artist: "周杰伦", Album: "范特西", Genre: "Pop", Language: "国语", Style: "抒情", Year: 2001, FilePath: "/m/b.mp3"},
		{Type: models.TypeLocal, Title: "C", Artist: "Beyond", Album: "海阔天空", Genre: "Rock", Language: "粤语", Year: 1993, FilePath: "/m/c.mp3"},
		{Type: models.TypeLocal, Title: "D", Artist: "Adele", Album: "21", Genre: "Pop", Language: "英语", Year: 2011, FilePath: "/m/d.mp3"},
		{Type: models.TypeLocal, Title: "E", Artist: "无标签", FilePath: "/m/e.mp3"}, // genre/year 空，不进 facet
	}
	if err := db.SongRepository().BatchCreate(ctx, songs); err != nil {
		t.Fatalf("BatchCreate: %v", err)
	}
	// 同步建立 song_artists 关联：新架构下歌手 facet/过滤走关联表，
	// seed 须把每首歌的 artist 写成 song_artists 行，否则 artist 维度为空。
	ar := db.SongArtistRepository()
	for _, s := range songs {
		if s.Artist == "" {
			continue
		}
		if err := ar.SetSongArtists(ctx, s.ID, []models.ArtistInput{
			{Name: s.Artist, Role: models.ArtistRoleArtist},
		}); err != nil {
			t.Fatalf("SetSongArtists %d (%q): %v", s.ID, s.Artist, err)
		}
	}
}

func TestListFacet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	seedFacetSongs(t, db)
	repo := db.SongRepository()
	ctx := context.Background()

	// genre：Pop=3, Rock=1，空值不计入；按计数降序。
	genres, err := repo.ListFacet(ctx, "genre", nil)
	if err != nil {
		t.Fatalf("facet genre: %v", err)
	}
	if len(genres) != 2 {
		t.Fatalf("expected 2 genres, got %d (%+v)", len(genres), genres)
	}
	if genres[0].Value != "Pop" || genres[0].Count != 3 {
		t.Fatalf("expected top genre Pop=3, got %+v", genres[0])
	}

	// language：国语=2, 粤语=1, 英语=1
	langs, err := repo.ListFacet(ctx, "language", nil)
	if err != nil {
		t.Fatalf("facet language: %v", err)
	}
	if len(langs) != 3 || langs[0].Value != "国语" || langs[0].Count != 2 {
		t.Fatalf("unexpected language facet: %+v", langs)
	}

	// style：R&B=1, 抒情=1（空值不计）
	styles, err := repo.ListFacet(ctx, "style", nil)
	if err != nil {
		t.Fatalf("facet style: %v", err)
	}
	if len(styles) != 2 {
		t.Fatalf("expected 2 styles, got %d (%+v)", len(styles), styles)
	}

	// year：2001=2, 1993=1, 2011=1（0 不计）
	years, err := repo.ListFacet(ctx, "year", nil)
	if err != nil {
		t.Fatalf("facet year: %v", err)
	}
	if len(years) != 3 {
		t.Fatalf("expected 3 years, got %d (%+v)", len(years), years)
	}

	// decade：2000=2, 1990=1, 2010=1
	decades, err := repo.ListFacet(ctx, "decade", nil)
	if err != nil {
		t.Fatalf("facet decade: %v", err)
	}
	if len(decades) != 3 {
		t.Fatalf("expected 3 decades, got %d (%+v)", len(decades), decades)
	}
	// 断言含 "2000" 且计数 2
	found := false
	for _, d := range decades {
		if d.Value == "2000" {
			found = true
			if d.Count != 2 {
				t.Fatalf("expected decade 2000 count 2, got %d", d.Count)
			}
		}
	}
	if !found {
		t.Fatalf("decade 2000 not found in %+v", decades)
	}

	// 未知维度返回 ErrNotFound
	if _, err := repo.ListFacet(ctx, "bogus", nil); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for unknown field, got %v", err)
	}
}

func TestListFacetSearchSortPaginate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	seedFacetSongs(t, db)
	repo := db.SongRepository()
	ctx := context.Background()

	// keyword 搜索：artist 含 "周" 只命中周杰伦
	got, err := repo.ListFacet(ctx, "artist", &FacetFilter{Keyword: "周"})
	if err != nil {
		t.Fatalf("facet artist keyword: %v", err)
	}
	if len(got) != 1 || got[0].Value != "周杰伦" || got[0].Count != 2 {
		t.Fatalf("expected only 周杰伦=2, got %+v", got)
	}

	// sort=name asc：artist 按名称升序（Adele/Beyond 在中文前）
	byName, err := repo.ListFacet(ctx, "artist", &FacetFilter{OrderBy: "name", Order: "asc"})
	if err != nil {
		t.Fatalf("facet artist by name: %v", err)
	}
	if len(byName) != 4 || byName[0].Value != "Adele" || byName[1].Value != "Beyond" {
		t.Fatalf("unexpected name-sorted artists: %+v", byName)
	}

	// 分页：limit=1 offset=0 → 只 1 条；CountFacet 返回去重总数 4
	page, err := repo.ListFacet(ctx, "artist", &FacetFilter{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("facet artist page: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 paged artist, got %d", len(page))
	}
	total, err := repo.CountFacet(ctx, "artist", "")
	if err != nil {
		t.Fatalf("count facet artist: %v", err)
	}
	if total != 4 {
		t.Fatalf("expected 4 distinct artists, got %d", total)
	}

	// CountFacet 带 keyword
	total, err = repo.CountFacet(ctx, "artist", "周")
	if err != nil {
		t.Fatalf("count facet artist keyword: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 distinct artist matching 周, got %d", total)
	}

	// 代表封面：本 seed 无封面 → cover_url 为空
	if page[0].CoverURL != "" {
		t.Fatalf("expected empty cover_url without cover, got %q", page[0].CoverURL)
	}

	// 未知维度
	if _, err := repo.CountFacet(ctx, "bogus", ""); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListDistinctNames(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	seedFacetSongs(t, db)
	repo := db.SongRepository()
	ctx := context.Background()

	// title：5 首歌名各不相同，去重后 5 条，按名称升序
	titles, err := repo.ListDistinctNames(ctx, "title")
	if err != nil {
		t.Fatalf("distinct title: %v", err)
	}
	if len(titles) != 5 {
		t.Fatalf("expected 5 titles, got %d (%+v)", len(titles), titles)
	}
	for i := 1; i < len(titles); i++ {
		if titles[i-1] > titles[i] {
			t.Fatalf("titles not ascending: %+v", titles)
		}
	}

	// artist：周杰伦重复应去重 → Adele/Beyond/周杰伦/无标签 共 4 条，整串不拆分
	artists, err := repo.ListDistinctNames(ctx, "artist")
	if err != nil {
		t.Fatalf("distinct artist: %v", err)
	}
	if len(artists) != 4 {
		t.Fatalf("expected 4 distinct artists, got %d (%+v)", len(artists), artists)
	}
	if artists[0] != "Adele" || artists[1] != "Beyond" {
		t.Fatalf("artists not ascending: %+v", artists)
	}

	// 未知维度返回 ErrNotFound
	if _, err := repo.ListDistinctNames(ctx, "album"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for unsupported field, got %v", err)
	}
}

func TestSongFilterByTag(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	seedFacetSongs(t, db)
	repo := db.SongRepository()
	ctx := context.Background()

	// genre=Pop → 3 首
	got, err := repo.List(ctx, &SongFilter{Genre: "Pop"})
	if err != nil {
		t.Fatalf("filter genre: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 Pop songs, got %d", len(got))
	}

	// artist=周杰伦 + language=国语 → 2 首（组合过滤）
	got, err = repo.List(ctx, &SongFilter{Artist: "周杰伦", Language: "国语"})
	if err != nil {
		t.Fatalf("filter artist+language: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 songs, got %d", len(got))
	}

	// decade=2000 → 2001 年的 2 首，不含 1993/2011
	got, err = repo.List(ctx, &SongFilter{DecadeStart: 2000})
	if err != nil {
		t.Fatalf("filter decade: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 songs in 2000s, got %d", len(got))
	}
	for _, s := range got {
		if s.Year < 2000 || s.Year >= 2010 {
			t.Fatalf("decade filter leaked year %d", s.Year)
		}
	}

	// year=1993 精确 → 1 首
	got, err = repo.List(ctx, &SongFilter{Year: 1993})
	if err != nil {
		t.Fatalf("filter year: %v", err)
	}
	if len(got) != 1 || got[0].Title != "C" {
		t.Fatalf("expected only song C for year 1993, got %+v", got)
	}

	// Count 与 List 共享过滤
	cnt, err := repo.Count(ctx, &SongFilter{Genre: "Pop"})
	if err != nil {
		t.Fatalf("count genre: %v", err)
	}
	if cnt != 3 {
		t.Fatalf("expected count 3, got %d", cnt)
	}
}

func TestMusicFacetExcludesRadioAndAudiobookSongs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "Music", Genre: "Spoken", FilePath: "/music/music.mp3"},
		{Type: models.TypeRadio, Title: "Radio", Genre: "Spoken", URL: "https://example.com/radio.m3u8"},
		{Type: models.TypeLocal, Title: "Book", Genre: "Spoken", FilePath: "/music/book.m4b"},
	}
	if err := db.SongRepository().BatchCreate(ctx, songs); err != nil {
		t.Fatalf("BatchCreate songs: %v", err)
	}
	book := &models.Playlist{
		Type:   models.PlaylistTypeNormal,
		Name:   "Book",
		Labels: []string{models.PlaylistLabelAudiobook},
	}
	if err := db.PlaylistRepository().Create(ctx, book); err != nil {
		t.Fatalf("create audiobook playlist: %v", err)
	}
	if err := db.PlaylistSongRepository().AddSong(ctx, book.ID, songs[2].ID, 0); err != nil {
		t.Fatalf("add audiobook song: %v", err)
	}

	filter := &FacetFilter{Type: models.TypeMusicQuery}
	facets, err := db.SongRepository().ListFacet(ctx, "genre", filter)
	if err != nil {
		t.Fatalf("list music facets: %v", err)
	}
	if len(facets) != 1 || facets[0].Count != 1 {
		t.Fatalf("music facets = %+v, want one value with count 1", facets)
	}
	total, err := db.SongRepository().CountFacetFiltered(ctx, "genre", filter)
	if err != nil {
		t.Fatalf("count music facets: %v", err)
	}
	if total != 1 {
		t.Fatalf("music facet total = %d, want 1", total)
	}
}

// seedTagFacetSongs 造 5 首本地歌（A 带封面）并挂上自定义标签，供 tag facet 测试复用。
// 返回 song 指针以便测试取用回填的 ID。
func seedTagFacetSongs(t *testing.T, db DB) []*models.Song {
	t.Helper()
	ctx := context.Background()
	songs := []*models.Song{
		{Type: models.TypeLocal, Title: "A", Artist: "周杰伦", FilePath: "/m/a.mp3", CoverPath: "/c/a.jpg"},
		{Type: models.TypeLocal, Title: "B", Artist: "周杰伦", FilePath: "/m/b.mp3"},
		{Type: models.TypeLocal, Title: "C", Artist: "Beyond", FilePath: "/m/c.mp3"},
		{Type: models.TypeLocal, Title: "D", Artist: "Adele", FilePath: "/m/d.mp3"},
		{Type: models.TypeLocal, Title: "E", Artist: "无标签", FilePath: "/m/e.mp3"},
	}
	if err := db.SongRepository().BatchCreate(ctx, songs); err != nil {
		t.Fatalf("BatchCreate: %v", err)
	}
	tagRepo := db.SongTagRepository()
	favID, err := tagRepo.Create(ctx, "fav", "#f00")
	if err != nil {
		t.Fatalf("create tag fav: %v", err)
	}
	cantID, err := tagRepo.Create(ctx, "cantopop", "#0f0")
	if err != nil {
		t.Fatalf("create tag cantopop: %v", err)
	}
	// 空标签（无关联歌曲）——内连接下不应出现在 facet。
	if _, err := tagRepo.Create(ctx, "solo", ""); err != nil {
		t.Fatalf("create tag solo: %v", err)
	}
	// fav → A、C；cantopop → C（同一首歌挂两个标签，验证 COUNT DISTINCT 不重复计）。
	links := []struct {
		songID int64
		tagID  int64
	}{
		{songs[0].ID, favID},  // A
		{songs[2].ID, favID},  // C
		{songs[2].ID, cantID}, // C
	}
	for _, l := range links {
		if err := tagRepo.LinkSongTag(ctx, l.songID, l.tagID); err != nil {
			t.Fatalf("link song %d tag %d: %v", l.songID, l.tagID, err)
		}
	}
	return songs
}

func TestListFacetTag(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	songs := seedTagFacetSongs(t, db)
	repo := db.SongRepository()
	ctx := context.Background()

	// tag facet：fav=2、cantopop=1；solo 无关联歌曲不出现。
	// 默认按计数降序 → fav(2) 在前。
	tags, err := repo.ListFacet(ctx, "tag", nil)
	if err != nil {
		t.Fatalf("facet tag: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d (%+v)", len(tags), tags)
	}
	if tags[0].Value != "fav" || tags[0].Count != 2 {
		t.Fatalf("expected top tag fav=2, got %+v", tags[0])
	}
	if tags[1].Value != "cantopop" || tags[1].Count != 1 {
		t.Fatalf("expected cantopop=1, got %+v", tags[1])
	}

	// 代表封面：fav 组内 A 带封面、C 无 → cover_url 指向 A。
	wantCover := fmt.Sprintf("/api/v1/songs/%d/cover", songs[0].ID)
	if tags[0].CoverURL != wantCover {
		t.Fatalf("expected fav cover %q, got %q", wantCover, tags[0].CoverURL)
	}
	// cantopop 组内只有 C（无封面）→ cover_url 为空。
	if tags[1].CoverURL != "" {
		t.Fatalf("expected empty cover for cantopop, got %q", tags[1].CoverURL)
	}

	// keyword 搜索：cantopop 含 "cant"。
	got, err := repo.ListFacet(ctx, "tag", &FacetFilter{Keyword: "cant"})
	if err != nil {
		t.Fatalf("facet tag keyword: %v", err)
	}
	if len(got) != 1 || got[0].Value != "cantopop" || got[0].Count != 1 {
		t.Fatalf("expected only cantopop=1, got %+v", got)
	}

	// sort=name asc：cantopop 在 fav 前（ASCII 升序）。
	byName, err := repo.ListFacet(ctx, "tag", &FacetFilter{OrderBy: "name", Order: "asc"})
	if err != nil {
		t.Fatalf("facet tag by name: %v", err)
	}
	if len(byName) != 2 || byName[0].Value != "cantopop" || byName[1].Value != "fav" {
		t.Fatalf("unexpected name-sorted tags: %+v", byName)
	}

	// 分页：limit=1 → 只 1 条；CountFacet 返回去重标签总数 2。
	page, err := repo.ListFacet(ctx, "tag", &FacetFilter{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("facet tag page: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("expected 1 paged tag, got %d", len(page))
	}
	total, err := repo.CountFacet(ctx, "tag", "")
	if err != nil {
		t.Fatalf("count facet tag: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 distinct tags, got %d", total)
	}
	// CountFacet 带 keyword
	total, err = repo.CountFacet(ctx, "tag", "fav")
	if err != nil {
		t.Fatalf("count facet tag keyword: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 distinct tag matching fav, got %d", total)
	}
}
