package server

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/kkdai/youtube/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestTemplates returns a minimal template map so handlers that call render do not panic.
func newTestTemplates() map[string]*template.Template {
	t := template.Must(template.New("").Parse("{{.}}"))
	return map[string]*template.Template{
		"index": t, "editSong": t, "newSong": t, "playVideo": t, "editRfid": t,
		"assignSong": t, "raw": t, "admin": t, "adminEditSong": t, "player": t, "print": t, "config": t,
	}
}

// noopPlayer is a test double that satisfies the player methods used in handlers.
func newNoopPlayer(t *testing.T) *player.Player {
	t.Helper()
	trueBin, err := exec.LookPath("true")
	require.NoError(t, err, "true binary not found in PATH")
	p, err := player.New(player.Config{FFPlayBin: trueBin, Beep: false}, log.NewNoOpLogger())
	require.NoError(t, err)
	return p
}

func TestJSONHandler(t *testing.T) {
	s := &Server{logger: log.NewNoOpLogger()}

	tests := []struct {
		name     string
		songID   string
		db       *db.MockDB
		wantBody func(t *testing.T, body []byte)
	}{
		{
			name:   "missing song_id",
			songID: "",
			db:     &db.MockDB{},
			wantBody: func(t *testing.T, body []byte) {
				var out map[string]string
				require.NoError(t, json.Unmarshal(body, &out))
				assert.Equal(t, "song_id required", out["error"])
			},
		},
		{
			name:   "song not found",
			songID: "nonexistent",
			db:     &db.MockDB{GetSongResult: nil, GetSongErr: db.ErrNotFound},
			wantBody: func(t *testing.T, body []byte) {
				var out map[string]string
				require.NoError(t, json.Unmarshal(body, &out))
				assert.Equal(t, "song not found", out["error"])
			},
		},
		{
			name:   "db error",
			songID: "some-id",
			db:     &db.MockDB{GetSongErr: assert.AnError},
			wantBody: func(t *testing.T, body []byte) {
				var out map[string]string
				require.NoError(t, json.Unmarshal(body, &out))
				assert.Equal(t, assert.AnError.Error(), out["error"])
			},
		},
		{
			name:   "success",
			songID: "song-123",
			db:     &db.MockDB{GetSongResult: &model.Song{ID: "song-123", Title: "Test Song"}},
			wantBody: func(t *testing.T, body []byte) {
				var song model.Song
				require.NoError(t, json.Unmarshal(body, &song))
				assert.Equal(t, "song-123", song.ID)
				assert.Equal(t, "Test Song", song.Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.db = tt.db
			req := httptest.NewRequest(http.MethodGet, "/song/"+tt.songID+"/json", nil)
			req.SetPathValue("song_id", tt.songID)
			w := httptest.NewRecorder()

			s.JSONHandler(w, req)

			res := w.Result()
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
			tt.wantBody(t, body)
		})
	}
}

func TestJSONGetSongByRFID(t *testing.T) {
	s := &Server{logger: log.NewNoOpLogger()}

	tests := []struct {
		name     string
		rfid     string
		db       *db.MockDB
		wantBody func(t *testing.T, body []byte)
	}{
		{
			name: "missing rfid",
			rfid: "",
			db:   &db.MockDB{},
			wantBody: func(t *testing.T, body []byte) {
				var out map[string]string
				require.NoError(t, json.Unmarshal(body, &out))
				assert.Equal(t, "rfid required", out["error"])
			},
		},
		{
			name: "rfid has no song",
			rfid: "rfid-123",
			db:   &db.MockDB{GetRFIDSongResult: &model.RFIDSong{RFID: "rfid-123", Songs: []string{}}},
			wantBody: func(t *testing.T, body []byte) {
				var out map[string]string
				require.NoError(t, json.Unmarshal(body, &out))
				assert.Equal(t, "rfid has no song", out["error"])
			},
		},
		{
			name: "GetRFIDSong error",
			rfid: "rfid-123",
			db:   &db.MockDB{GetRFIDSongErr: assert.AnError},
			wantBody: func(t *testing.T, body []byte) {
				var out map[string]string
				require.NoError(t, json.Unmarshal(body, &out))
				assert.Equal(t, assert.AnError.Error(), out["error"])
			},
		},
		{
			name: "success",
			rfid: "rfid-123",
			db: &db.MockDB{
				GetRFIDSongResult: &model.RFIDSong{RFID: "rfid-123", Songs: []string{"song-1"}},
				GetSongResult:     &model.Song{ID: "song-1", Title: "RFID Song"},
			},
			wantBody: func(t *testing.T, body []byte) {
				var song model.Song
				require.NoError(t, json.Unmarshal(body, &song))
				assert.Equal(t, "song-1", song.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.db = tt.db
			req := httptest.NewRequest(http.MethodGet, "/"+tt.rfid+"/json", nil)
			req.SetPathValue("rfid", tt.rfid)
			w := httptest.NewRecorder()

			s.JSONGetSongByRFID(w, req)

			res := w.Result()
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
			tt.wantBody(t, body)
		})
	}
}

func TestDeleteSongHandler(t *testing.T) {
	s := &Server{logger: log.NewNoOpLogger()}

	tests := []struct {
		name         string
		songID       string
		db           *db.MockDB
		wantRedirect string
		wantStatus   int
	}{
		{
			name:       "missing song_id",
			songID:     "",
			db:         &db.MockDB{},
			wantStatus: http.StatusOK,
		},
		{
			name:       "db error",
			songID:     "song-123",
			db:         &db.MockDB{DeleteSongErr: assert.AnError},
			wantStatus: http.StatusOK,
		},
		{
			name:         "success",
			songID:       "song-123",
			db:           &db.MockDB{},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.db = tt.db
			req := httptest.NewRequest(http.MethodDelete, "/song/"+tt.songID, nil)
			req.SetPathValue("song_id", tt.songID)
			w := httptest.NewRecorder()

			s.DeleteSongHandler(w, req)

			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, tt.wantStatus, res.StatusCode)
			if tt.wantRedirect != "" {
				assert.Equal(t, tt.wantRedirect, res.Header.Get("Location"))
			}
		})
	}
}

func TestListSongHandler(t *testing.T) {
	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	tests := []struct {
		name      string
		setupDB   func(t *testing.T) db.DBer
		wantErr   bool
		bodyCheck func(t *testing.T, body []byte)
	}{
		{
			name: "success with songs",
			setupDB: func(t *testing.T) db.DBer {
				d := initDB(t)
				require.NoError(t, d.UpdateSong(&model.Song{ID: "test_song", RFID: "test_song_rfid"}))
				return d
			},
			wantErr: false,
			bodyCheck: func(t *testing.T, body []byte) {
				assert.NotEmpty(t, body)
			},
		},
		{
			name: "ListSongs error",
			setupDB: func(t *testing.T) db.DBer {
				return &db.MockDB{ListSongsErr: assert.AnError}
			},
			wantErr: true,
		},
		{
			name: "ListRFIDSongs error",
			setupDB: func(t *testing.T) db.DBer {
				return &db.MockDB{ListSongsResult: []*model.Song{}, ListRFIDSongsErr: assert.AnError}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newNoopPlayer(t)
			s := &Server{
				db:         tt.setupDB(t),
				logger:     log.NewNoOpLogger(),
				player:     p,
				downloader: &downloader.MockDownloader{Response: map[string]*youtube.Video{}},
				templates:  newTestTemplates(),
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			s.ListSongHandler(w, req)

			res := w.Result()
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			if tt.wantErr {
				assert.Contains(t, string(body), "ListSongHandler")
			} else {
				assert.Equal(t, http.StatusOK, res.StatusCode)
			}
			if tt.bodyCheck != nil {
				tt.bodyCheck(t, body)
			}
		})
	}
}

func TestDownloadSong(t *testing.T) {
	downloadURL := "https://example.com/watch?v=xyz"
	mockVideo := &youtube.Video{
		ID: "test-id", Title: "Downloaded Song Title",
		Thumbnails: youtube.Thumbnails{{URL: "https://thumb.example.com/img.jpg"}},
	}

	tests := []struct {
		name         string
		form         map[string]string
		dl           *downloader.MockDownloader
		wantRedirect string
		wantStatus   int
		waitCreate   bool
		checkDB      func(t *testing.T, mock *db.MockDB)
	}{
		{
			name:       "ParseForm error",
			form:       nil,
			dl:         &downloader.MockDownloader{Response: map[string]*youtube.Video{downloadURL: mockVideo}},
			wantStatus: http.StatusOK,
		},
		{
			name:         "success redirects and creates song in background",
			form:         map[string]string{"url": downloadURL, "force": "1"},
			dl:           &downloader.MockDownloader{Response: map[string]*youtube.Video{downloadURL: mockVideo}},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			waitCreate:   true,
			checkDB: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.CreateSongCalls, 1)
				assert.Equal(t, "Downloaded Song Title", mock.CreateSongCalls[0].Title)
			},
		},
		{
			name:         "rfid strips colons and assigns",
			form:         map[string]string{"url": downloadURL, "force": "1", "rfid": "AB:CD:EF"},
			dl:           &downloader.MockDownloader{Response: map[string]*youtube.Video{downloadURL: mockVideo}},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			waitCreate:   true,
			checkDB: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.AddRFIDSongCalls, 1)
				assert.Equal(t, "ABCDEF", mock.AddRFIDSongCalls[0].RFID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &db.MockDB{}
			if tt.form != nil && tt.form["rfid"] != "" {
				mockDB.GetRFIDSongErr = db.ErrNotFound
			}
			var createDone chan struct{}
			if tt.waitCreate {
				createDone = make(chan struct{})
				mockDB.OnCreateSong = func(*model.Song) { close(createDone) }
			}

			s := &Server{db: mockDB, logger: log.NewNoOpLogger(), downloader: tt.dl}

			var req *http.Request
			if tt.form != nil {
				body := strings.NewReader(urlValuesFromMap(tt.form).Encode())
				req = httptest.NewRequest(http.MethodPost, "/download", body)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(http.MethodPost, "/download", errReader(0))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}

			w := httptest.NewRecorder()
			s.DownloadSong(w, req)

			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, tt.wantStatus, res.StatusCode)
			if tt.wantRedirect != "" {
				assert.Equal(t, tt.wantRedirect, res.Header.Get("Location"))
			}
			if tt.waitCreate {
				<-createDone
			}
			if tt.checkDB != nil {
				tt.checkDB(t, mockDB)
			}
		})
	}
}

func TestNewSongHandler(t *testing.T) {
	newSongURL := "https://example.com/v"
	mockVideo := &youtube.Video{
		ID: "new-id", Title: "New Song Title",
		Thumbnails: youtube.Thumbnails{{URL: "https://thumb.example.com/new.jpg"}},
	}

	tests := []struct {
		name         string
		form         map[string]string
		dl           *downloader.MockDownloader
		wantRedirect string
		wantStatus   int
		checkCalls   func(t *testing.T, mock *db.MockDB)
	}{
		{
			name:         "success creates song and redirects",
			form:         map[string]string{"url": newSongURL, "force": "1"},
			dl:           &downloader.MockDownloader{Response: map[string]*youtube.Video{newSongURL: mockVideo}},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			checkCalls: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.UpdateSongCalls, 1)
				assert.Equal(t, "New Song Title", mock.UpdateSongCalls[0].Title)
			},
		},
		{
			name:         "rfid strips colons",
			form:         map[string]string{"url": newSongURL, "force": "1", "rfid": "AA:BB:CC"},
			dl:           &downloader.MockDownloader{Response: map[string]*youtube.Video{newSongURL: mockVideo}},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			checkCalls: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.AddRFIDSongCalls, 1)
				assert.Equal(t, "AABBCC", mock.AddRFIDSongCalls[0].RFID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &db.MockDB{}
			s := &Server{db: mockDB, logger: log.NewNoOpLogger(), downloader: tt.dl}

			req := newMultipartRequest(t, http.MethodPost, "/song", tt.form)
			w := httptest.NewRecorder()
			s.NewSongHandler(w, req)

			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, tt.wantStatus, res.StatusCode)
			assert.Equal(t, tt.wantRedirect, res.Header.Get("Location"))
			if tt.checkCalls != nil {
				tt.checkCalls(t, mockDB)
			}
		})
	}
}

func TestRedownloadSongAssetsHandler(t *testing.T) {
	t.Run("does not redownload when files exist", func(t *testing.T) {
		tempDir := t.TempDir()
		videoPath := filepath.Join(tempDir, "video.mp4")
		thumbPath := filepath.Join(tempDir, "thumb.jpg")
		require.NoError(t, os.WriteFile(videoPath, []byte("video"), 0o600))
		require.NoError(t, os.WriteFile(thumbPath, []byte("thumb"), 0o600))

		mockDB := &db.MockDB{
			GetSongResult: &model.Song{
				ID:        "song-1",
				URL:       "https://example.com/watch?v=ok",
				FilePath:  videoPath,
				Thumbnail: thumbPath,
			},
		}
		s := &Server{
			db:         mockDB,
			logger:     log.NewNoOpLogger(),
			downloader: &downloader.MockDownloader{Response: map[string]*youtube.Video{}},
		}

		req := httptest.NewRequest(http.MethodGet, "/song/song-1/redownload", nil)
		req.SetPathValue("song_id", "song-1")
		w := httptest.NewRecorder()

		s.RedownloadSongAssetsHandler(w, req)

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/songs", res.Header.Get("Location"))
		assert.Len(t, mockDB.UpdateSongCalls, 0)
	})

	t.Run("redownloads missing video and thumbnail", func(t *testing.T) {
		downloadURL := "https://example.com/watch?v=missing-both"
		mockDB := &db.MockDB{
			GetSongResult: &model.Song{
				ID:        "song-2",
				URL:       downloadURL,
				FilePath:  "missing-video.mp4",
				Thumbnail: "missing-thumb.jpg",
			},
		}
		s := &Server{
			db:     mockDB,
			logger: log.NewNoOpLogger(),
			downloader: &downloader.MockDownloader{Response: map[string]*youtube.Video{
				downloadURL: {
					Title:      "downloaded-video-path.mp4",
					Thumbnails: youtube.Thumbnails{{URL: "https://thumb.example.com/new.jpg"}},
				},
			}},
		}

		req := httptest.NewRequest(http.MethodGet, "/song/song-2/redownload", nil)
		req.SetPathValue("song_id", "song-2")
		w := httptest.NewRecorder()

		s.RedownloadSongAssetsHandler(w, req)

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/songs", res.Header.Get("Location"))
		require.Len(t, mockDB.UpdateSongCalls, 1)
		assert.Equal(t, filepath.ToSlash(filepath.Join("song_files", "downloaded-video-path.mp4")), mockDB.UpdateSongCalls[0].FilePath)
		assert.Equal(t, filepath.ToSlash(filepath.Join("thumb_files", "new.jpg")), mockDB.UpdateSongCalls[0].Thumbnail)
	})

	t.Run("redownloads only thumbnail when video exists", func(t *testing.T) {
		tempDir := t.TempDir()
		videoPath := filepath.Join(tempDir, "video.mp4")
		require.NoError(t, os.WriteFile(videoPath, []byte("video"), 0o600))

		downloadURL := "https://example.com/watch?v=missing-thumb"
		mockDB := &db.MockDB{
			GetSongResult: &model.Song{
				ID:        "song-3",
				URL:       downloadURL,
				FilePath:  videoPath,
				Thumbnail: "missing-thumb.jpg",
			},
		}
		s := &Server{
			db:     mockDB,
			logger: log.NewNoOpLogger(),
			downloader: &downloader.MockDownloader{Response: map[string]*youtube.Video{
				downloadURL: {
					Title:      "unused-video-title",
					Thumbnails: youtube.Thumbnails{{URL: "https://thumb.example.com/thumb-only.jpg"}},
				},
			}},
		}

		req := httptest.NewRequest(http.MethodGet, "/song/song-3/redownload", nil)
		req.SetPathValue("song_id", "song-3")
		w := httptest.NewRecorder()

		s.RedownloadSongAssetsHandler(w, req)

		res := w.Result()
		defer res.Body.Close()
		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/songs", res.Header.Get("Location"))
		require.Len(t, mockDB.UpdateSongCalls, 1)
		assert.Equal(t, videoPath, mockDB.UpdateSongCalls[0].FilePath)
		assert.Equal(t, filepath.ToSlash(filepath.Join("thumb_files", "thumb-only.jpg")), mockDB.UpdateSongCalls[0].Thumbnail)
	})
}

// --- helpers ---

func urlValuesFromMap(m map[string]string) url.Values {
	vals := make(url.Values)
	for k, v := range m {
		vals.Set(k, v)
	}
	return vals
}

type errReader int

func (errReader) Read(_ []byte) (int, error) { return 0, assert.AnError }

func newMultipartRequest(t *testing.T, method, path string, form map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range form {
		_ = w.WriteField(k, v)
	}
	contentType := w.FormDataContentType()
	require.NoError(t, w.Close())
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", contentType)
	return req
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func initDB(t *testing.T) db.DBer {
	moduleRoot := findModuleRoot(t)
	require.NoError(t, os.Chdir(moduleRoot))
	_ = os.Remove("test.db")
	d, err := db.NewSongDB("test.db")
	require.NoError(t, err)
	return d
}
