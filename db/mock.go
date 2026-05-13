package db

import (
	"sync"

	"github.com/jaredwarren/rpi_music/model"
)

// MockDB is a test double for DBer used in table-driven handler tests.
// Set the desired return values on the struct before passing to handlers.
// CreateSongCalls and UpdateSongCalls record arguments for assertions.
// OnCreateSong and OnUpdateSong are optional callbacks (e.g. to unblock tests waiting on async handlers).
type MockDB struct {
	mu sync.RWMutex

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
	OnAddRFIDSong    func(rfid, songID string)
}

// AddRFIDSongCall records arguments passed to AddRFIDSong.
type AddRFIDSongCall struct {
	RFID   string
	SongID string
}

func (m *MockDB) Close() error { return nil }
func (m *MockDB) GetSong(id string) (*model.Song, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.GetSongResult, m.GetSongErr
}
func (m *MockDB) ListSongs() ([]*model.Song, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ListSongsResult, m.ListSongsErr
}
func (m *MockDB) CreateSong(song *model.Song) error {
	m.mu.Lock()
	m.CreateSongCalls = append(m.CreateSongCalls, song)
	onCreate := m.OnCreateSong
	m.mu.Unlock()
	if onCreate != nil {
		onCreate(song)
	}
	return nil
}
func (m *MockDB) UpdateSong(song *model.Song) error {
	m.mu.Lock()
	m.UpdateSongCalls = append(m.UpdateSongCalls, song)
	onUpdate := m.OnUpdateSong
	m.mu.Unlock()
	if onUpdate != nil {
		onUpdate(song)
	}
	return nil
}
func (m *MockDB) DeleteSong(id string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.DeleteSongErr
}
func (m *MockDB) SongExists(id string) (bool, error) { return false, nil }
func (m *MockDB) GetRFIDSong(rfid string) (*model.RFIDSong, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.GetRFIDSongResult, m.GetRFIDSongErr
}
func (m *MockDB) GetSongRFID(songID string) (*model.RFIDSong, error) { return nil, nil }
func (m *MockDB) AddRFIDSong(rfid, songID string) error {
	m.mu.Lock()
	m.AddRFIDSongCalls = append(m.AddRFIDSongCalls, AddRFIDSongCall{RFID: rfid, SongID: songID})
	onAdd := m.OnAddRFIDSong
	m.mu.Unlock()
	if onAdd != nil {
		onAdd(rfid, songID)
	}
	return nil
}
func (m *MockDB) RemoveRFIDSong(rfid, songID string) error { return nil }
func (m *MockDB) DeleteRFID(id string) error               { return nil }
func (m *MockDB) ListRFIDSongs() ([]*model.RFIDSong, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ListRFIDSongsResult, m.ListRFIDSongsErr
}
func (m *MockDB) RFIDExists(rfid string) (bool, error)   { return false, nil }
func (m *MockDB) DeleteSongFromRFID(songID string) error { return nil }

func (m *MockDB) UpdateSongCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.UpdateSongCalls)
}

func (m *MockDB) LastUpdateSongCall() *model.Song {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.UpdateSongCalls) == 0 {
		return nil
	}
	return m.UpdateSongCalls[len(m.UpdateSongCalls)-1]
}

func (m *MockDB) AddRFIDSongCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.AddRFIDSongCalls)
}

func (m *MockDB) LastAddRFIDSongCall() (AddRFIDSongCall, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.AddRFIDSongCalls) == 0 {
		return AddRFIDSongCall{}, false
	}
	return m.AddRFIDSongCalls[len(m.AddRFIDSongCalls)-1], true
}

func (m *MockDB) CreateSongCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.CreateSongCalls)
}

func (m *MockDB) LastCreateSongCall() *model.Song {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.CreateSongCalls) == 0 {
		return nil
	}
	return m.CreateSongCalls[len(m.CreateSongCalls)-1]
}
