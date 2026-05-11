package main

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/jaredwarren/rpi_music/player"
	"github.com/jaredwarren/rpi_music/rfid"
	"github.com/stretchr/testify/require"
)

func TestRunRFIDLoopIncrementsPlaysAndUpdatesSong(t *testing.T) {
	trueBin, err := exec.LookPath("true")
	require.NoError(t, err)

	p, err := player.New(player.Config{
		FFPlayBin: trueBin,
		Beep:      false,
	}, log.NewNoOpLogger())
	require.NoError(t, err)

	song := &model.Song{
		ID:       "song-1",
		FilePath: "song_files/test.mp3",
		Plays:    2,
	}
	mockDB := &db.MockDB{
		GetRFIDSongResult: &model.RFIDSong{RFID: "UID123", Songs: []string{"song-1"}},
		GetSongResult:     song,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan rfid.Event, 1)
	go runRFIDLoop(ctx, events, mockDB, p, log.NewNoOpLogger())
	events <- rfid.Event{UID: "UID123"}

	require.Eventually(t, func() bool {
		return len(mockDB.UpdateSongCalls) > 0
	}, time.Second, 10*time.Millisecond)

	require.Equal(t, 1, len(mockDB.UpdateSongCalls))
	require.Equal(t, 3, mockDB.UpdateSongCalls[0].Plays)
}
