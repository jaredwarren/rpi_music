package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSongRFIDIndexLifecycle(t *testing.T) {
	d := newTestDB(t)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	require.NoError(t, d.AddRFIDSong("rfid-1", "song-1"))

	rs, err := d.GetSongRFID("song-1")
	require.NoError(t, err)
	require.Equal(t, "rfid-1", rs.RFID)
	require.Equal(t, []string{"song-1"}, rs.Songs)

	require.NoError(t, d.DeleteSongFromRFID("song-1"))
	_, err = d.GetSongRFID("song-1")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestDeleteRFIDRemovesSongIndexEntries(t *testing.T) {
	d := newTestDB(t)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	require.NoError(t, d.AddRFIDSong("rfid-1", "song-1"))
	require.NoError(t, d.AddRFIDSong("rfid-1", "song-2"))

	require.NoError(t, d.DeleteRFID("rfid-1"))

	_, err := d.GetSongRFID("song-1")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = d.GetSongRFID("song-2")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRemoveRFIDSongRemovesSongIndexEntry(t *testing.T) {
	d := newTestDB(t)
	t.Cleanup(func() { require.NoError(t, d.Close()) })

	require.NoError(t, d.AddRFIDSong("rfid-1", "song-1"))
	require.NoError(t, d.RemoveRFIDSong("rfid-1", "song-1"))

	_, err := d.GetSongRFID("song-1")
	require.ErrorIs(t, err, ErrNotFound)
}

func newTestDB(t *testing.T) DBer {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := NewSongDB(dbPath)
	require.NoError(t, err)
	return d
}
