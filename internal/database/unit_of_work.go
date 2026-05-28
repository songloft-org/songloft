package database

// UnitOfWork 把多个 Repository 绑定到同一个事务。
// 通过 Transactor.RunInTx 获得，用于跨表原子写入
// （如 convert_service 的 CreateSong + ReplaceSongInPlaylist）。
type UnitOfWork struct {
	Songs         *SongRepository
	Playlists     *PlaylistRepository
	PlaylistSongs *PlaylistSongRepository
}
