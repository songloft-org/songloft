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
