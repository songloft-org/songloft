package models

import (
	"testing"
)

// TestSongValidate 测试 Song 验证逻辑
func TestSongValidate(t *testing.T) {
	tests := []struct {
		name    string
		song    Song
		wantErr bool
	}{
		{
			name: "valid local song",
			song: Song{
				Type:     TypeLocal,
				Title:    "测试歌曲",
				FilePath: "/music/test.mp3",
			},
			wantErr: false,
		},
		{
			name: "valid remote song",
			song: Song{
				Type:  TypeRemote,
				Title: "网络歌曲",
				URL:   "https://example.com/song.mp3",
			},
			wantErr: false,
		},
		{
			name: "valid radio",
			song: Song{
				Type:   TypeRadio,
				Title:  "测试电台",
				URL:    "https://example.com/radio.m3u8",
				IsLive: true,
			},
			wantErr: false,
		},
		{
			name: "missing title",
			song: Song{
				Type:     TypeLocal,
				FilePath: "/music/test.mp3",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			song: Song{
				Type:  "invalid",
				Title: "测试",
			},
			wantErr: true,
		},
		{
			name: "local song without file path",
			song: Song{
				Type:  TypeLocal,
				Title: "测试",
			},
			wantErr: true,
		},
		{
			name: "remote song without url",
			song: Song{
				Type:  TypeRemote,
				Title: "测试",
			},
			wantErr: true,
		},
		{
			name: "plugin-sourced remote song without url is valid",
			song: Song{
				Type:            TypeRemote,
				Title:           "插件歌曲",
				PluginEntryPath: "subsonic",
				SourceData:      `{"id":"x1"}`,
			},
			wantErr: false,
		},
		{
			name: "remote song with plugin_entry_path but no source_data is invalid",
			song: Song{
				Type:            TypeRemote,
				Title:           "缺 source_data",
				PluginEntryPath: "subsonic",
			},
			wantErr: true,
		},
		{
			name:    "radio",
			song:    Song{Type: TypeRadio, Title: "测试电台", URL: "https://example.com/radio.m3u8"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.song.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Song.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSongIsLocal 测试判断是否为本地歌曲
func TestSongIsLocal(t *testing.T) {
	tests := []struct {
		name string
		song Song
		want bool
	}{
		{
			name: "local song",
			song: Song{Type: TypeLocal},
			want: true,
		},
		{
			name: "remote song",
			song: Song{Type: TypeRemote},
			want: false,
		},
		{
			name: "radio",
			song: Song{Type: TypeRadio},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.song.IsLocal(); got != tt.want {
				t.Errorf("Song.IsLocal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSongIsRadio 测试判断是否为电台/广播
func TestSongIsRadio(t *testing.T) {
	tests := []struct {
		name string
		song Song
		want bool
	}{
		{
			name: "radio",
			song: Song{Type: TypeRadio},
			want: true,
		},
		{
			name: "local song",
			song: Song{Type: TypeLocal},
			want: false,
		},
		{
			name: "remote song",
			song: Song{Type: TypeRemote},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.song.IsRadio(); got != tt.want {
				t.Errorf("Song.IsRadio() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSongPlaybackURL_HLSRadioGetsM3U8Suffix 验证 HLS 电台 PlaybackURL 带 .m3u8 后缀。
// 没有这个后缀，ExoPlayer/AVPlayer 会选 ProgressiveMediaSource 而非 HlsMediaSource，
// 导致 player 用错协议解析 m3u8 文本，电台无法播放（不开 HLS 代理时也会出现该问题）。
func TestSongPlaybackURL_HLSRadioGetsM3U8Suffix(t *testing.T) {
	cases := []struct {
		name string
		song Song
		want string
	}{
		{
			name: "HLS radio (.m3u8)",
			song: Song{ID: 42, Type: TypeRadio, URL: "https://cdn.example/live/x.m3u8"},
			want: "/api/v1/songs/42/play.m3u8",
		},
		{
			name: "HLS radio (.m3u)",
			song: Song{ID: 42, Type: TypeRadio, URL: "https://cdn.example/live/x.m3u"},
			want: "/api/v1/songs/42/play.m3u8",
		},
		{
			name: "HLS radio with query string",
			song: Song{ID: 7, Type: TypeRadio, URL: "https://cdn.example/live/x.m3u8?token=abc"},
			want: "/api/v1/songs/7/play.m3u8",
		},
		{
			name: "non-HLS radio (icecast mp3 stream)",
			song: Song{ID: 5, Type: TypeRadio, URL: "https://stream.example/live.mp3"},
			want: "/api/v1/songs/5/play",
		},
		{
			name: "local song",
			song: Song{ID: 1, Type: TypeLocal, FilePath: "/music/a.mp3"},
			want: "/api/v1/songs/1/play",
		},
		{
			name: "remote song",
			song: Song{ID: 2, Type: TypeRemote, URL: "https://x/y.mp3"},
			want: "/api/v1/songs/2/play",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.song.PlaybackURL(); got != c.want {
				t.Errorf("PlaybackURL() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestLyricURLPath(t *testing.T) {
	cases := []struct {
		name string
		song Song
		want string
	}{
		{
			name: "ID 为 0 返回空",
			song: Song{ID: 0, Type: TypeLocal},
			want: "",
		},
		{
			name: "local 有歌词",
			song: Song{ID: 1, Type: TypeLocal, Lyric: `{"lyric":"[00:00.00]hi"}`},
			want: "/api/v1/songs/1/lyric",
		},
		{
			name: "local 无歌词",
			song: Song{ID: 1, Type: TypeLocal},
			want: "",
		},
		{
			name: "remote 无歌词也返回 URL（触发插件搜索）",
			song: Song{ID: 5, Type: TypeRemote},
			want: "/api/v1/songs/5/lyric",
		},
		{
			name: "remote 有歌词",
			song: Song{ID: 5, Type: TypeRemote, Lyric: `{"lyric":"[00:00.00]hi"}`},
			want: "/api/v1/songs/5/lyric",
		},
		{
			name: "radio 无歌词",
			song: Song{ID: 3, Type: TypeRadio},
			want: "",
		},
		{
			name: "lyric_source=url 有远程 URL",
			song: Song{ID: 7, Type: TypeLocal, LyricSource: LyricSourceURL, LyricRemoteURL: "https://example.com/lyric"},
			want: "/api/v1/songs/7/lyric",
		},
		{
			name: "lyric_source=url 但远程 URL 为空",
			song: Song{ID: 7, Type: TypeLocal, LyricSource: LyricSourceURL},
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.song.LyricURLPath(); got != c.want {
				t.Errorf("LyricURLPath() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestLyricURLPathWithProvider 验证 #303：存在歌词提供者插件时，
// 本地无歌词歌曲也放行歌词 URL，从而触发客户端请求 → 后端自动搜索。
func TestLyricURLPathWithProvider(t *testing.T) {
	orig := HasLyricProvider
	t.Cleanup(func() { HasLyricProvider = orig })

	localNoLyric := Song{ID: 1, Type: TypeLocal}

	// 无 hook（等价于没装歌词插件）：保持历史行为，返回空。
	HasLyricProvider = nil
	if got := localNoLyric.LyricURLPath(); got != "" {
		t.Errorf("无歌词插件时 local 无歌词应返回空, got %q", got)
	}

	// hook 报告无提供者：仍返回空。
	HasLyricProvider = func() bool { return false }
	if got := localNoLyric.LyricURLPath(); got != "" {
		t.Errorf("无提供者时 local 无歌词应返回空, got %q", got)
	}

	// hook 报告有提供者：放行歌词 URL 以触发自动搜索。
	HasLyricProvider = func() bool { return true }
	if got := localNoLyric.LyricURLPath(); got != "/api/v1/songs/1/lyric" {
		t.Errorf("有提供者时 local 无歌词应放行歌词 URL, got %q", got)
	}

	// 有提供者也不影响 radio（radio 不参与歌词搜索）。
	radio := Song{ID: 3, Type: TypeRadio}
	if got := radio.LyricURLPath(); got != "" {
		t.Errorf("radio 应始终返回空, got %q", got)
	}
}

// TestPlaylistValidate 测试 Playlist 验证逻辑
func TestPlaylistValidate(t *testing.T) {
	tests := []struct {
		name     string
		playlist Playlist
		wantErr  bool
	}{
		{
			name: "valid normal playlist",
			playlist: Playlist{
				Type: PlaylistTypeNormal,
				Name: "我的歌单",
			},
			wantErr: false,
		},
		{
			name: "valid radio playlist",
			playlist: Playlist{
				Type: PlaylistTypeRadio,
				Name: "我的电台",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			playlist: Playlist{
				Type: PlaylistTypeNormal,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			playlist: Playlist{
				Type: "invalid",
				Name: "测试",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.playlist.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Playlist.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPlaylistValidateForUpdate 测试 Playlist 更新验证逻辑（不校验 type）
func TestPlaylistValidateForUpdate(t *testing.T) {
	tests := []struct {
		name     string
		playlist Playlist
		wantErr  bool
	}{
		{
			name: "valid update with name only",
			playlist: Playlist{
				Name: "我的歌单",
			},
			wantErr: false,
		},
		{
			name: "valid update ignores type field",
			playlist: Playlist{
				Type: "invalid_type",
				Name: "我的歌单",
			},
			wantErr: false,
		},
		{
			name:     "missing name",
			playlist: Playlist{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.playlist.ValidateForUpdate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Playlist.ValidateForUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPlaylistCanAddSong 测试歌单类型约束
func TestPlaylistCanAddSong(t *testing.T) {
	tests := []struct {
		name         string
		playlistType string
		songType     string
		want         bool
	}{
		{
			name:         "normal playlist can add local song",
			playlistType: PlaylistTypeNormal,
			songType:     TypeLocal,
			want:         true,
		},
		{
			name:         "normal playlist can add remote song",
			playlistType: PlaylistTypeNormal,
			songType:     TypeRemote,
			want:         true,
		},
		{
			name:         "normal playlist cannot add radio",
			playlistType: PlaylistTypeNormal,
			songType:     TypeRadio,
			want:         false,
		},
		{
			name:         "radio playlist can add radio",
			playlistType: PlaylistTypeRadio,
			songType:     TypeRadio,
			want:         true,
		},
		{
			name:         "radio playlist cannot add local song",
			playlistType: PlaylistTypeRadio,
			songType:     TypeLocal,
			want:         false,
		},
		{
			name:         "radio playlist cannot add remote song",
			playlistType: PlaylistTypeRadio,
			songType:     TypeRemote,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playlist := Playlist{Type: tt.playlistType}
			if got := playlist.CanAddSong(tt.songType); got != tt.want {
				t.Errorf("Playlist.CanAddSong() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPlaylistSongValidate 测试 PlaylistSong 验证逻辑
func TestPlaylistSongValidate(t *testing.T) {
	tests := []struct {
		name         string
		playlistSong PlaylistSong
		wantErr      bool
	}{
		{
			name: "valid playlist song",
			playlistSong: PlaylistSong{
				PlaylistID: 1,
				SongID:     1,
				Position:   1,
			},
			wantErr: false,
		},
		{
			name: "missing playlist id",
			playlistSong: PlaylistSong{
				SongID:   1,
				Position: 1,
			},
			wantErr: true,
		},
		{
			name: "missing song id",
			playlistSong: PlaylistSong{
				PlaylistID: 1,
				Position:   1,
			},
			wantErr: true,
		},
		{
			name: "invalid position",
			playlistSong: PlaylistSong{
				PlaylistID: 1,
				SongID:     1,
				Position:   0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.playlistSong.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PlaylistSong.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfigValidate 测试 Config 验证逻辑
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Key:   "test_key",
				Value: `{"path": "music"}`,
			},
			wantErr: false,
		},
		{
			name: "missing key",
			config: Config{
				Value: `{"path": "music"}`,
			},
			wantErr: true,
		},
		{
			name: "missing value",
			config: Config{
				Key: "test_key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
