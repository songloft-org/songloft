package models

// SongMetadataMask records which values were manually edited, including empty values.
type SongMetadataMask int64

// These bits are stored in songs.metadata_override_mask. Keep their values stable
// and keep the background updates in queries/songs.sql in sync.
const (
	MetadataOverrideTitle SongMetadataMask = 1 << iota
	MetadataOverrideArtist
	MetadataOverrideAlbum
	MetadataOverrideYear
	MetadataOverrideGenre
	MetadataOverrideLanguage
	MetadataOverrideStyle
	MetadataOverrideTrack
	MetadataOverrideDisc
	MetadataOverrideLyrics
	MetadataOverridePicture
)

func (m SongMetadataMask) Has(field SongMetadataMask) bool {
	return m&field != 0
}
