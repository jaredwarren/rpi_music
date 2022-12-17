package db

import (
	"github.com/google/uuid"
	"github.com/jaredwarren/rpi_music/model"
	"github.com/spf13/viper"
)

func Up(db DBer) error {
	version := viper.GetInt("db.version")
	if version >= 2 {
		return nil
	}

	viper.Set("db.version", 2)
	err := viper.WriteConfig()
	if err != nil {
		return err
	}

	oldSongs, err := db.OldListSongs()
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
		err := db.UpdateSong(newSong)
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
