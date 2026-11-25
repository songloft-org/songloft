package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/hanxi/tag"

	"songloft/internal/database"
	"songloft/internal/fileutil"
	"songloft/internal/httputil"
	"songloft/internal/models"
)

type LocalFileFailure struct {
	State string
	error
}

func (s *SongService) checkLocalSource(ctx context.Context, library string, song *models.Song) (os.FileInfo, *LocalFileFailure) {
	if song.Type != models.TypeLocal || song.FilePath == "" {
		return nil, &LocalFileFailure{"changed", errors.New("song is no longer local")}
	}
	count, err := s.songs.CountSongsByFilePath(ctx, song.FilePath)
	if err != nil {
		return nil, &LocalFileFailure{"error", err}
	}
	if song.CueSourcePath != "" || count != 1 {
		return nil, &LocalFileFailure{"shared", errors.New("CUE or shared audio file")}
	}
	if err := localDirectory(library, filepath.Dir(song.FilePath), false); err != nil {
		return nil, &LocalFileFailure{"changed", err}
	}
	info, err := os.Lstat(song.FilePath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, &LocalFileFailure{"changed", errors.New("source file is missing or is not a regular file")}
	}
	if !fileModTimeMatches(song.FileModifiedAt, info.ModTime()) {
		return nil, &LocalFileFailure{"changed", errors.New("source file changed since its last library scan")}
	}
	return info, nil
}

func checkLocalContainer(path string) *LocalFileFailure {
	f, err := os.Open(path)
	if err != nil {
		return &LocalFileFailure{"error", err}
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if errors.Is(err, tag.ErrNotID3v1) {
		// An untagged MP3 can still accept ID3v2.
		return nil
	}
	if err != nil {
		return &LocalFileFailure{"unsupported", errors.New("native tags are unavailable for this file")}
	}
	if provider, ok := m.(interface{ CUESheetBlock() *tag.CUESheetData }); ok && provider.CUESheetBlock() != nil {
		return &LocalFileFailure{"shared", errors.New("embedded CUE sheet")}
	}
	switch m.FileType() {
	case tag.MP3, tag.FLAC, tag.M4A, tag.M4B, tag.MOV, tag.ALAC,
		tag.AAC, tag.ASF, tag.AVI, tag.DSF, tag.DFF, tag.FLV, tag.MKA, tag.RM, tag.WAV, tag.WEBM, tag.WV,
		tag.OGG, tag.APE, tag.AIFF:
		return nil
	default:
		return &LocalFileFailure{"unsupported", errors.New("incremental tags are unavailable for this format")}
	}
}

// Symlinked subdirectories alias different logical library paths. Each local
// only works with the indexed paths of physical directories within this root.
func localDirectory(library, dir string, create bool) error {
	relative, err := filepath.Rel(library, dir)
	if err != nil || !filepath.IsLocal(relative) {
		return errors.New("directory is outside the music library")
	}
	if relative == "." {
		return nil
	}
	parent := library
	for part := range strings.SplitSeq(relative, string(filepath.Separator)) {
		parent = filepath.Join(parent, part)
		info, err := os.Lstat(parent)
		if create && errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(parent, 0755); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
			info, err = os.Lstat(parent)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("library path contains a symlink or a non-directory")
		}
	}
	return nil
}

func sameLocalFile(path string, previous os.FileInfo) bool {
	current, err := os.Lstat(path)
	return err == nil && os.SameFile(current, previous) && current.Size() == previous.Size() &&
		current.Mode() == previous.Mode() && current.ModTime().Equal(previous.ModTime())
}

type localReader struct {
	ctx context.Context
	io.Reader
}

func (r localReader) Read(buf []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(buf)
}

func copyLocalSource(ctx context.Context, out io.Writer, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, hash), localReader{ctx, f})
	return hex.EncodeToString(hash.Sum(nil)), err
}

func hashLocalSource(ctx context.Context, path string) (string, error) {
	return copyLocalSource(ctx, io.Discard, path)
}

func localCheckTarget(source, target string) error {
	if source == target {
		return nil
	}
	_, err := os.Lstat(target)
	switch {
	case err == nil:
		return os.ErrExist
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return err
	}
}

func localSidecarTarget(source, sidecar, target string) string {
	if sidecar == "" || source == target {
		return ""
	}
	if strings.HasPrefix(sidecar, source+".") {
		return target + sidecar[len(source):]
	}
	stem := strings.TrimSuffix(source, filepath.Ext(source))
	return strings.TrimSuffix(target, filepath.Ext(target)) + sidecar[len(stem):]
}

// LocalMetadataUpdate carries tag values and a destination relative to the library.
type LocalMetadataUpdate struct {
	SongID     int64               `json:"song_id"`
	Revision   string              `json:"revision"`
	TargetPath string              `json:"target_path"`
	Title      string              `json:"title"`
	Artists    []string            `json:"artists"`
	Album      string              `json:"album"`
	Year       int                 `json:"year"`
	Track      string              `json:"track"`
	ISRC       string              `json:"isrc"`
	Lyrics     models.LyricPayload `json:"lyrics"`
	CoverURL   string              `json:"cover_url"`
}

type LocalMetadataResult struct {
	Path    string `json:"path"`
	Target  string `json:"target"`
	State   string `json:"state"`
	Detail  string `json:"detail,omitempty"`
	Warning string `json:"warning,omitempty"`
}

func (s *SongService) WriteLocalMetadata(ctx context.Context, update LocalMetadataUpdate) LocalMetadataResult {
	return s.writeLocalMetadata(ctx, update, localArtworkClient)
}

func (s *SongService) writeLocalMetadata(ctx context.Context, update LocalMetadataUpdate, client *http.Client) LocalMetadataResult {
	var result LocalMetadataResult
	fail := func(state string, err error) LocalMetadataResult {
		result.State, result.Detail = state, err.Error()
		return result
	}
	if update.SongID <= 0 || update.Revision == "" || s.tx == nil {
		return fail("error", errors.New("invalid local metadata update"))
	}
	relative := filepath.FromSlash(update.TargetPath)
	if !filepath.IsLocal(relative) || relative == "." || filepath.Clean(relative) != relative {
		return fail("error", errors.New("invalid library-relative destination"))
	}
	library, err := s.resolveMusicPath()
	if err != nil {
		return fail("changed", err)
	}
	library = filepath.Clean(library)
	lock := &s.metadataEditLocks[uint64(update.SongID)%uint64(len(s.metadataEditLocks))]
	lock.Lock()
	defer lock.Unlock()
	if err := ctx.Err(); err != nil {
		return fail("error", err)
	}
	song, err := s.songs.GetByID(ctx, update.SongID)
	if err != nil {
		return fail("changed", errors.New("the song or its path changed"))
	}
	result.Path = song.FilePath
	before, failure := s.checkLocalSource(ctx, library, song)
	if failure != nil {
		return fail(failure.State, failure)
	}
	if failure := checkLocalContainer(song.FilePath); failure != nil {
		return fail(failure.State, failure)
	}
	target := filepath.Join(library, relative)
	if filepath.Ext(song.FilePath) != filepath.Ext(target) {
		return fail("error", errors.New("file extension mismatch"))
	}
	result.Target = target
	if err := localCheckTarget(song.FilePath, target); err != nil {
		return fail(localFileErrorState(err), err)
	}
	sidecar, _ := FindSidecarLyricFile(song.FilePath)
	sidecarTarget := localSidecarTarget(song.FilePath, sidecar, target)
	if sidecarTarget != "" {
		if err := localCheckTarget(sidecar, sidecarTarget); err != nil {
			return fail(localFileErrorState(err), fmt.Errorf("target lyrics: %w", err))
		}
	}
	if err := localDirectory(library, filepath.Dir(target), true); err != nil {
		return fail("error", err)
	}
	stage, err := os.CreateTemp(filepath.Dir(target), ".songloft-tag-*.tmp")
	if err != nil {
		return fail("error", err)
	}
	defer os.Remove(stage.Name())
	digest, err := copyLocalSource(ctx, stage, song.FilePath)
	if err = errors.Join(err, stage.Chmod(before.Mode().Perm()), stage.Close()); err != nil {
		return fail("error", err)
	}
	if localFileRevision(song, before, digest) != update.Revision || !sameLocalFile(song.FilePath, before) {
		return fail("changed", errors.New("source file or metadata changed after inspection"))
	}

	opts, metadata := s.localTagOptions(ctx, song, update, sidecar, &result, client)
	hasTags := opts.Title != "" || opts.Artist != "" || opts.Album != "" || opts.Year > 0 ||
		opts.Track != "" || opts.Lyrics != "" || opts.Picture != nil
	if hasTags {
		if err := tag.WriteTag(stage.Name(), opts); err != nil {
			state := "error"
			if errors.Is(err, tag.ErrUnsupportedWrite) {
				state = "unsupported"
			}
			return fail(state, err)
		}
		if err := verifyLocalTags(stage.Name(), opts); err != nil {
			return fail("unsupported", err)
		}
	}
	stagedInfo, err := os.Stat(stage.Name())
	if err != nil {
		return fail("error", err)
	}
	var artists []string
	if opts.Artist != "" {
		artists = update.Artists
	}
	if err := s.commitLocal(ctx, library, song, metadata, artists, target, stage.Name(), sidecar, sidecarTarget, before, stagedInfo); err != nil {
		var failure *LocalFileFailure
		if errors.As(err, &failure) {
			return fail(failure.State, failure)
		}
		return fail(localFileErrorState(err), err)
	}
	result.State = "written"
	return result
}

func localFileErrorState(err error) string {
	if errors.Is(err, os.ErrExist) {
		return "conflict"
	}
	return "error"
}

func (s *SongService) localTagOptions(ctx context.Context, song *models.Song, update LocalMetadataUpdate, sidecar string,
	result *LocalMetadataResult, client *http.Client) (tag.WriteOptions, models.Song) {
	out := *song
	opts := tag.WriteOptions{Incremental: true}
	mask := song.MetadataOverrideMask
	if update.Title != "" && !mask.Has(models.MetadataOverrideTitle) {
		opts.Title, out.Title = update.Title, update.Title
	}
	if !mask.Has(models.MetadataOverrideArtist) {
		if artist := database.JoinDisplayArtists(update.Artists); artist != "" {
			opts.Artist, out.Artist = artist, artist
		}
	}
	if update.Album != "" && !mask.Has(models.MetadataOverrideAlbum) {
		opts.Album, out.Album = update.Album, update.Album
	}
	if update.Year >= 1000 && update.Year <= 9999 && !mask.Has(models.MetadataOverrideYear) {
		opts.Year, out.Year = update.Year, update.Year
	}
	if track := normalizeTrack(update.Track); track != "" && !mask.Has(models.MetadataOverrideTrack) {
		opts.Track, out.Track = track, track
	}
	if out.ISRC == "" {
		out.ISRC = update.ISRC
	}
	if strings.TrimSpace(update.Lyrics.Lyric) != "" && allowsLocalLyrics(song, sidecar) {
		opts.Lyrics, out.Lyric = update.Lyrics.Lyric, update.Lyrics.MarshalString()
		out.LyricSource, out.LyricRemoteURL = models.LyricSourceScraped, ""
	}
	if update.CoverURL != "" && allowsLocalCover(song) {
		imageFile, path, err := s.localCover(ctx, client, update.CoverURL)
		if err != nil {
			result.Warning = "cover download failed: " + err.Error()
		} else {
			opts.Picture, out.CoverPath = imageFile, path
		}
	}
	return opts, out
}

var localArtworkClient = newLocalCoverClient()

func localPublicIPs(ctx context.Context, host string) ([]net.IP, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !ip.IsGlobalUnicast() || isPrivateIP(ip) {
			return nil, errors.New("non-public cover address")
		}
	}
	return ips, nil
}

func newLocalCoverClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		if (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.User != nil || req.URL.Hostname() == "" {
			return nil, errors.New("invalid public cover URL")
		}
		if _, err := localPublicIPs(req.Context(), req.URL.Hostname()); err != nil {
			return nil, err
		}
		return httputil.ProxyFromGlobal(req)
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if proxy, _ := url.Parse(httputil.GetGlobalProxy()); proxy != nil && host == proxy.Hostname() {
			return dialer.DialContext(ctx, network, address)
		}
		ips, err := localPublicIPs(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, errors.New("could not connect to a public cover address")
	}
	return &http.Client{Timeout: 20 * time.Second, Transport: transport, CheckRedirect: func(_ *http.Request, redirects []*http.Request) error {
		if len(redirects) >= 5 {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}

func (s *SongService) localCover(ctx context.Context, client *http.Client, coverURL string) (*tag.Picture, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return nil, "", errors.New("invalid public cover URL")
	}
	req.Header.Set("Accept", "image/jpeg,image/png")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", errors.New("cover provider image request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("cover provider HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (10<<20)+1))
	if err != nil || len(data) > 10<<20 {
		return nil, "", errors.New("cover could not be read or exceeds 10 MiB")
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (kind != "jpeg" && kind != "png") || cfg.Width <= 0 || cfg.Height <= 0 ||
		cfg.Width > 8192 || cfg.Height > 8192 {
		return nil, "", errors.New("cover must be JPEG or PNG")
	}
	ext := kind
	if kind == "jpeg" {
		ext = "jpg"
	}
	path, err := s.SaveCoverFromData(data, ext)
	if err != nil {
		return nil, "", err
	}
	return &tag.Picture{MIMEType: tag.MIMETypeFromExt("." + ext), Data: data}, path, nil
}

func verifyLocalTags(path string, opts tag.WriteOptions) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		return err
	}
	if opts.Title != "" && m.Title() != opts.Title || opts.Artist != "" && m.Artist() != opts.Artist ||
		opts.Album != "" && m.Album() != opts.Album || opts.Year != 0 && m.Year() != opts.Year ||
		opts.Track != "" && canonicalTrack(m.Track()) != opts.Track ||
		opts.Lyrics != "" && m.Lyrics() != opts.Lyrics ||
		opts.Picture != nil && !pictureEqual(m.Picture(), opts.Picture) {
		return errors.New("native tags did not round-trip safely")
	}
	if provider, ok := m.(interface{ CUESheetBlock() *tag.CUESheetData }); ok && provider.CUESheetBlock() != nil {
		return errors.New("embedded CUE sheet")
	}
	return nil
}

func (s *SongService) commitLocal(ctx context.Context, root string, song *models.Song, metadata models.Song, artists []string,
	target, stage, sidecar, sidecarTarget string, before, stagedInfo os.FileInfo) error {
	s.localWriteMu.Lock()
	defer s.localWriteMu.Unlock()
	switch s.GetScanProgress().Status {
	case ScanStatusScanning, ScanStatusImporting, ScanStatusSplittingCue, ScanStatusCreatingPlaylists, ScanStatusCancelling:
		return &LocalFileFailure{"error", errors.New("a library scan is in progress")}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Once publication begins, finish it or roll it back before honouring cancellation.
	ctx = context.WithoutCancel(ctx)
	onDisk := song.FilePath
	move := onDisk != target
	var backup string
	var installed, lyricCopied bool
	defer func() {
		if backup != "" {
			_ = os.Remove(backup)
		}
	}()
	err := s.tx.RunInTx(ctx, func(ctx context.Context, uow *database.UnitOfWork) error {
		current, err := uow.Songs.GetByID(ctx, song.ID)
		if err != nil {
			return err
		}
		count, err := uow.Songs.CountSongsByFilePath(ctx, onDisk)
		if err != nil {
			return err
		}
		library, rootErr := s.resolveMusicPath()
		if rootErr != nil || filepath.Clean(library) != root || current.FilePath != onDisk ||
			current.Type != models.TypeLocal || current.MetadataEditedAt != song.MetadataEditedAt ||
			current.MetadataOverrideMask != song.MetadataOverrideMask || !sameLocalFile(onDisk, before) {
			return &LocalFileFailure{"changed", errors.New("source file or metadata changed before publication")}
		}
		if count != 1 || current.CueSourcePath != "" {
			return &LocalFileFailure{"shared", errors.New("CUE or shared audio file")}
		}
		if err := localDirectory(root, filepath.Dir(target), false); err != nil {
			return err
		}
		if move {
			count, err := uow.Songs.CountSongsByFilePath(ctx, target)
			if err != nil {
				return err
			}
			if count != 0 {
				return fmt.Errorf("target already indexed: %w", os.ErrExist)
			}
			if err := fileutil.LinkOrCopyExclusive(stage, target); err != nil {
				return err
			}
			installed = true
		}
		if sidecarTarget != "" && sidecarTarget != sidecar {
			info, err := os.Lstat(sidecar)
			if err != nil || !info.Mode().IsRegular() {
				return errors.New("sidecar lyrics changed")
			}
			if err := fileutil.CopyFileExclusive(sidecar, sidecarTarget); err != nil {
				return fmt.Errorf("preserve sidecar lyrics: %w", err)
			}
			lyricCopied = true
			if !sameLocalFile(sidecar, info) {
				return &LocalFileFailure{"changed", errors.New("sidecar lyrics changed")}
			}
		}
		if !sameLocalFile(onDisk, before) {
			return &LocalFileFailure{"changed", errors.New("source file changed before publication")}
		}
		backup, err = localBackupName(onDisk)
		if err != nil {
			return err
		}
		if move {
			if err := fileutil.MoveFile(onDisk, backup); err != nil {
				backup = ""
				return err
			}
		} else {
			if err := fileutil.LinkOrCopyExclusive(onDisk, backup); err != nil {
				backup = ""
				return err
			}
			if err := fileutil.MoveFile(stage, target); err != nil {
				return err
			}
			installed = true
		}
		if len(artists) > 0 {
			current.Artist = metadata.Artist
			if err := localSetArtists(ctx, uow, current.ID, artists); err != nil {
				return err
			}
		}
		current.Title, current.Album, current.Year, current.Track = metadata.Title, metadata.Album, metadata.Year, metadata.Track
		current.Lyric, current.LyricSource, current.LyricRemoteURL = metadata.Lyric, metadata.LyricSource, metadata.LyricRemoteURL
		current.CoverPath, current.ISRC = metadata.CoverPath, metadata.ISRC
		current.FilePath = target
		current.FileSize = stagedInfo.Size()
		modified := stagedInfo.ModTime()
		if move {
			if info, err := os.Stat(target); err == nil {
				modified = info.ModTime()
			}
		}
		current.FileModifiedAt = &modified
		return uow.Songs.Update(ctx, current)
	})
	if err == nil {
		return nil
	}
	if backup != "" && (move || installed) {
		if restoreErr := fileutil.MoveFile(backup, onDisk); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore original from %s: %w", backup, restoreErr))
			backup = ""
		}
	}
	if installed && move {
		err = errors.Join(err, os.Remove(target))
	}
	if lyricCopied {
		err = errors.Join(err, os.Remove(sidecarTarget))
	}
	return err
}

func localBackupName(path string) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".songloft-local-backup-*.tmp")
	if err != nil {
		return "", err
	}
	return f.Name(), errors.Join(f.Close(), os.Remove(f.Name()))
}

func localSetArtists(ctx context.Context, uow *database.UnitOfWork, songID int64, artists []string) error {
	existing, err := uow.SongArtists.GetBySongID(ctx, songID)
	if err != nil {
		return err
	}
	inputs := slices.Clone(artistInputsFromMetadata(&Metadata{Artists: artists}))
	for _, artist := range existing {
		if artist.Role != models.ArtistRoleArtist {
			inputs = append(inputs, models.ArtistInput{Name: artist.Artist.Name, Role: artist.Role, Position: artist.Position})
		}
	}
	return uow.SongArtists.SetSongArtists(ctx, songID, inputs)
}
