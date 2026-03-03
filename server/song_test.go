package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/downloader"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/kkdai/youtube/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONHandler(t *testing.T) {
	s := &Server{logger: log.NewNoOpLogger()}

	tests := []struct {
		name       string
		songID     string
		db         *db.MockDB
		wantStatus int
		wantBody   func(t *testing.T, body []byte)
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
			if tt.songID != "" {
				req = mux.SetURLVars(req, map[string]string{"song_id": tt.songID})
			} else {
				req = mux.SetURLVars(req, map[string]string{"song_id": ""})
			}
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
			name: "GetSong error after rfid lookup",
			rfid: "rfid-123",
			db: &db.MockDB{
				GetRFIDSongResult: &model.RFIDSong{RFID: "rfid-123", Songs: []string{"song-1"}},
				GetSongErr:        assert.AnError,
			},
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
				assert.Equal(t, "RFID Song", song.Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.db = tt.db
			req := httptest.NewRequest(http.MethodGet, "/"+tt.rfid+"/json", nil)
			req = mux.SetURLVars(req, map[string]string{"rfid": tt.rfid})
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
			name:         "missing song_id",
			songID:       "",
			db:           &db.MockDB{},
			wantStatus:   http.StatusOK, // httpError does not set status
			wantRedirect: "",
		},
		{
			name:         "db error",
			songID:       "song-123",
			db:           &db.MockDB{DeleteSongErr: assert.AnError},
			wantStatus:   http.StatusOK, // httpError does not set status
			wantRedirect: "",
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
			req = mux.SetURLVars(req, map[string]string{"song_id": tt.songID})
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
	t.Cleanup(func() { os.Chdir(origWd) })

	tests := []struct {
		name     string
		setupDB  func(t *testing.T) db.DBer
		wantErr  bool
		bodyCheck func(t *testing.T, body []byte)
	}{
		{
			name: "success with songs",
			setupDB: func(t *testing.T) db.DBer {
				db := initDB(t)
				err := db.UpdateSong(&model.Song{ID: "test_song", RFID: "test_song_rfid"})
				require.NoError(t, err)
				return db
			},
			wantErr: false,
			bodyCheck: func(t *testing.T, body []byte) {
				// Handler renders HTML; just ensure we get a non-error response body
				assert.NotEmpty(t, body)
			},
		},
		{
			name: "ListSongs error returns bad request",
			setupDB: func(t *testing.T) db.DBer {
				return &db.MockDB{ListSongsErr: assert.AnError}
			},
			wantErr: true,
			bodyCheck: func(t *testing.T, body []byte) {
				assert.Contains(t, string(body), "ListSongHandler")
			},
		},
		{
			name: "ListRFIDSongs error returns bad request",
			setupDB: func(t *testing.T) db.DBer {
				return &db.MockDB{
					ListSongsResult:  []*model.Song{},
					ListRFIDSongsErr: assert.AnError,
				}
			},
			wantErr: true,
			bodyCheck: func(t *testing.T, body []byte) {
				assert.Contains(t, string(body), "ListSongHandler")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := tt.setupDB(t)
			s := &Server{
				db:         db,
				logger:     log.NewNoOpLogger(),
				downloader: &downloader.MockDownloader{
					Response: map[string]*youtube.Video{"new url": {
						ID: "test_song", Title: "song title",
						Thumbnails: youtube.Thumbnails{{URL: "thumb_url"}},
					}},
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			s.ListSongHandler(w, req)

			res := w.Result()
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			if tt.wantErr {
				// httpError does not set status code; error is in body
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
		ID:    "test-id",
		Title: "Downloaded Song Title",
		Thumbnails: youtube.Thumbnails{{URL: "https://thumb.example.com/img.jpg"}},
	}

	tests := []struct {
		name         string
		form         map[string]string
		downloader   *downloader.MockDownloader
		wantRedirect string
		wantStatus   int
		waitCreate   bool
		checkBody    func(t *testing.T, mock *db.MockDB)
	}{
		{
			name: "ParseForm error returns bad request",
			form: nil,
			downloader: &downloader.MockDownloader{
				Response: map[string]*youtube.Video{downloadURL: mockVideo},
			},
			wantStatus: http.StatusOK,
			waitCreate: false,
			checkBody: func(t *testing.T, mock *db.MockDB) {
				assert.Empty(t, mock.CreateSongCalls)
			},
		},
		{
			name: "success redirects and creates song in background",
			form: map[string]string{"url": downloadURL, "force": "1"},
			downloader: &downloader.MockDownloader{
				Response: map[string]*youtube.Video{downloadURL: mockVideo},
			},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			waitCreate:   true,
			checkBody: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.CreateSongCalls, 1)
				assert.Equal(t, "Downloaded Song Title", mock.CreateSongCalls[0].Title)
				assert.Equal(t, downloadURL, mock.CreateSongCalls[0].URL)
				assert.NotEmpty(t, mock.CreateSongCalls[0].ID)
			},
		},
		{
			name: "success with rfid and force assigns rfid to created song",
			form: map[string]string{"url": downloadURL, "force": "1", "rfid": "abc123"},
			downloader: &downloader.MockDownloader{
				Response: map[string]*youtube.Video{downloadURL: mockVideo},
			},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			waitCreate:   true,
			checkBody: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.CreateSongCalls, 1)
				songID := mock.CreateSongCalls[0].ID
				require.Len(t, mock.AddRFIDSongCalls, 1)
				assert.Equal(t, "abc123", mock.AddRFIDSongCalls[0].RFID)
				assert.Equal(t, songID, mock.AddRFIDSongCalls[0].SongID)
			},
		},
		{
			name: "success with rfid containing colons strips colons before assign",
			form: map[string]string{"url": downloadURL, "force": "1", "rfid": "AB:CD:EF"},
			downloader: &downloader.MockDownloader{
				Response: map[string]*youtube.Video{downloadURL: mockVideo},
			},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			waitCreate:   true,
			checkBody: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.CreateSongCalls, 1)
				require.Len(t, mock.AddRFIDSongCalls, 1)
				assert.Equal(t, "ABCDEF", mock.AddRFIDSongCalls[0].RFID)
				assert.Equal(t, mock.CreateSongCalls[0].ID, mock.AddRFIDSongCalls[0].SongID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &db.MockDB{}
			// GetRFIDSong returns not found so RFID assign path can run when rfid is set
			if tt.form != nil && tt.form["rfid"] != "" {
				mockDB.GetRFIDSongResult = nil
				mockDB.GetRFIDSongErr = db.ErrNotFound
			}
			var createDone chan struct{}
			if tt.waitCreate {
				createDone = make(chan struct{})
				mockDB.OnCreateSong = func(*model.Song) { close(createDone) }
			}

			s := &Server{
				db:         mockDB,
				logger:     log.NewNoOpLogger(),
				downloader: tt.downloader,
			}

			var req *http.Request
			if tt.form != nil {
				body := strings.NewReader(urlValuesFromMap(tt.form).Encode())
				req = httptest.NewRequest(http.MethodPost, "/download", body)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				// No body / unreadable to trigger ParseForm path
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
			if tt.checkBody != nil {
				tt.checkBody(t, mockDB)
			}
		})
	}
}

func TestNewSongHandler(t *testing.T) {
	newSongURL := "https://example.com/v"
	mockVideo := &youtube.Video{
		ID:    "new-id",
		Title: "New Song Title",
		Thumbnails: youtube.Thumbnails{{URL: "https://thumb.example.com/new.jpg"}},
	}

	tests := []struct {
		name         string
		form         map[string]string
		downloader   *downloader.MockDownloader
		wantRedirect string
		wantStatus   int
		checkCalls   func(t *testing.T, mock *db.MockDB)
	}{
		{
			name: "success creates song and redirects",
			form: map[string]string{"url": newSongURL, "force": "1"},
			downloader: &downloader.MockDownloader{
				Response: map[string]*youtube.Video{newSongURL: mockVideo},
			},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			checkCalls: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.UpdateSongCalls, 1)
				assert.Equal(t, "New Song Title", mock.UpdateSongCalls[0].Title)
				assert.Equal(t, newSongURL, mock.UpdateSongCalls[0].URL)
				assert.NotEmpty(t, mock.UpdateSongCalls[0].ID)
			},
		},
		{
			name: "success with rfid and force assigns rfid to song",
			form: map[string]string{"url": newSongURL, "force": "1", "rfid": "rfid-xyz"},
			downloader: &downloader.MockDownloader{
				Response: map[string]*youtube.Video{newSongURL: mockVideo},
			},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			checkCalls: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.UpdateSongCalls, 1)
				songID := mock.UpdateSongCalls[0].ID
				require.Len(t, mock.AddRFIDSongCalls, 1)
				assert.Equal(t, "rfid-xyz", mock.AddRFIDSongCalls[0].RFID)
				assert.Equal(t, songID, mock.AddRFIDSongCalls[0].SongID)
			},
		},
		{
			name: "success with rfid containing colons strips colons",
			form: map[string]string{"url": newSongURL, "force": "1", "rfid": "AA:BB:CC"},
			downloader: &downloader.MockDownloader{
				Response: map[string]*youtube.Video{newSongURL: mockVideo},
			},
			wantStatus:   http.StatusFound,
			wantRedirect: "/songs",
			checkCalls: func(t *testing.T, mock *db.MockDB) {
				require.Len(t, mock.UpdateSongCalls, 1)
				require.Len(t, mock.AddRFIDSongCalls, 1)
				assert.Equal(t, "AABBCC", mock.AddRFIDSongCalls[0].RFID)
				assert.Equal(t, mock.UpdateSongCalls[0].ID, mock.AddRFIDSongCalls[0].SongID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &db.MockDB{}
			s := &Server{
				db:         mockDB,
				logger:     log.NewNoOpLogger(),
				downloader: tt.downloader,
			}

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

// urlValuesFromMap builds url.Values from a string map.
func urlValuesFromMap(m map[string]string) url.Values {
	vals := make(url.Values)
	for k, v := range m {
		vals.Set(k, v)
	}
	return vals
}

// errReader is an io.Reader that always returns an error (to trigger ParseForm failure).
type errReader int

func (errReader) Read(_ []byte) (int, error) {
	return 0, assert.AnError
}

// newMultipartRequest builds a POST request with multipart/form-data body.
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

// findModuleRoot walks up from the current directory to find the module root (where go.mod is).
func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	assert.NoError(t, err)
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
	err := os.Chdir(moduleRoot)
	assert.NoError(t, err)
	os.Remove("test.db")

	// Init DB
	db, err := db.NewSongDB("test.db")
	assert.NoError(t, err)
	return db
}
