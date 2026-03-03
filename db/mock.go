package db

import "github.com/jaredwarren/rpi_music/model"

// MockDB is a test double for DBer used in table-driven handler tests.
// Set the desired return values on the struct before passing to handlers.
// CreateSongCalls and UpdateSongCalls record arguments for assertions.
// OnCreateSong and OnUpdateSong are optional callbacks (e.g. to unblock tests waiting on async handlers).
type MockDB struct {
	GetSongResult       *model.Song
	GetSongErr          error
	GetRFIDSongResult   *model.RFIDSong
	GetRFIDSongErr      error
	ListSongsResult     []*model.Song
	ListSongsErr        error
	ListRFIDSongsResult []*model.RFIDSong
	ListRFIDSongsErr    error
	DeleteSongErr       error

	CreateSongCalls  []*model.Song
	UpdateSongCalls  []*model.Song
	AddRFIDSongCalls []AddRFIDSongCall
	OnCreateSong     func(*model.Song)
	OnUpdateSong     func(*model.Song)
}

// AddRFIDSongCall records arguments passed to AddRFIDSong.
type AddRFIDSongCall struct {
	RFID   string
	SongID string
}

func (m *MockDB) Close() error                           { return nil }
func (m *MockDB) GetSong(id string) (*model.Song, error) { return m.GetSongResult, m.GetSongErr }
func (m *MockDB) ListSongs() ([]*model.Song, error)      { return m.ListSongsResult, m.ListSongsErr }
func (m *MockDB) CreateSong(song *model.Song) error {
	m.CreateSongCalls = append(m.CreateSongCalls, song)
	if m.OnCreateSong != nil {
		m.OnCreateSong(song)
	}
	return nil
}
func (m *MockDB) UpdateSong(song *model.Song) error {
	m.UpdateSongCalls = append(m.UpdateSongCalls, song)
	if m.OnUpdateSong != nil {
		m.OnUpdateSong(song)
	}
	return nil
}
func (m *MockDB) DeleteSong(id string) error         { return m.DeleteSongErr }
func (m *MockDB) SongExists(id string) (bool, error) { return false, nil }
func (m *MockDB) GetRFIDSong(rfid string) (*model.RFIDSong, error) {
	return m.GetRFIDSongResult, m.GetRFIDSongErr
}
func (m *MockDB) GetSongRFID(songID string) (*model.RFIDSong, error) { return nil, nil }
func (m *MockDB) AddRFIDSong(rfid, songID string) error {
	m.AddRFIDSongCalls = append(m.AddRFIDSongCalls, AddRFIDSongCall{RFID: rfid, SongID: songID})
	return nil
}
func (m *MockDB) RemoveRFIDSong(rfid, songID string) error { return nil }
func (m *MockDB) DeleteRFID(id string) error               { return nil }
func (m *MockDB) ListRFIDSongs() ([]*model.RFIDSong, error) {
	return m.ListRFIDSongsResult, m.ListRFIDSongsErr
}
func (m *MockDB) RFIDExists(rfid string) (bool, error)   { return false, nil }
func (m *MockDB) DeleteSongFromRFID(songID string) error { return nil }
