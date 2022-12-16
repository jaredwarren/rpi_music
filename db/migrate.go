package db

import (
	"github.com/google/uuid"
	"github.com/jaredwarren/rpi_music/model"
)

func Up(db DBer) error {

	// loop each db song
	// // save to new bucket rfid as key
	return nil

	oldSongs, err := db.ListSongs()
	if err != nil {
		return err
	}

	for _, v := range oldSongs {
		newSongID := uuid.New().String()
		newSong := &model.Song{
			ID:        newSongID,
			Thumbnail: v.Thumbnail,
			Title:     v.Title,
			URL:       v.URL,
			FilePath:  v.FilePath,
		}
		err := db.UpdateSongV2(newSong)
		if err != nil {
			return err
		}

		if v.RFID != "" {
			err = db.AddRFIDSong(v.RFID, newSongID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func Down(db DBer) error {

	return nil
}
