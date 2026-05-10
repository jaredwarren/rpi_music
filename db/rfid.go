package db

import (
	"encoding/json"
	"fmt"

	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

const RFIDBucket = "RFIDBucket"

// RFIDStore is the read/write interface for RFID→song mappings.
type RFIDStore interface {
	GetRFIDSong(rfid string) (*model.RFIDSong, error)
	GetSongRFID(songID string) (*model.RFIDSong, error)
	AddRFIDSong(rfid, songID string) error
	RemoveRFIDSong(rfid, songID string) error
	DeleteRFID(id string) error
	ListRFIDSongs() ([]*model.RFIDSong, error)
	RFIDExists(rfid string) (bool, error)
	DeleteSongFromRFID(songID string) error
}

func (s *SongDB) RFIDExists(rfid string) (bool, error) {
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		exists = b.Get([]byte(rfid)) != nil
		return nil
	})
	return exists, err
}

func (s *SongDB) ListRFIDSongs() ([]*model.RFIDSong, error) {
	var out []*model.RFIDSong
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		return b.ForEach(func(k, v []byte) error {
			var rs model.RFIDSong
			if err := json.Unmarshal(v, &rs); err != nil {
				return err
			}
			out = append(out, &rs)
			return nil
		})
	})
	return out, err
}

func (s *SongDB) GetRFIDSong(rfid string) (*model.RFIDSong, error) {
	var rs *model.RFIDSong
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		v := b.Get([]byte(rfid))
		if v == nil {
			return ErrNotFound
		}
		rs = &model.RFIDSong{}
		return json.Unmarshal(v, rs)
	})
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (s *SongDB) AddRFIDSong(rfid, songID string) error {
	if rfid == "" || songID == "" {
		return fmt.Errorf("rfid and songID required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		v := b.Get([]byte(rfid))
		if v == nil {
			buf, err := json.Marshal(&model.RFIDSong{
				RFID:  rfid,
				Songs: []string{songID},
			})
			if err != nil {
				return err
			}
			return b.Put([]byte(rfid), buf)
		}

		var rs model.RFIDSong
		if err := json.Unmarshal(v, &rs); err != nil {
			return err
		}
		for _, id := range rs.Songs {
			if id == songID {
				return nil // already present
			}
		}
		rs.Songs = append(rs.Songs, songID)
		buf, err := json.Marshal(&rs)
		if err != nil {
			return err
		}
		return b.Put([]byte(rfid), buf)
	})
}

func (s *SongDB) RemoveRFIDSong(rfid, songID string) error {
	if rfid == "" || songID == "" {
		return fmt.Errorf("rfid and songID required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		v := b.Get([]byte(rfid))
		if v == nil {
			return nil // no such RFID, idempotent
		}

		var rs model.RFIDSong
		if err := json.Unmarshal(v, &rs); err != nil {
			return err
		}
		for i, id := range rs.Songs {
			if id == songID {
				rs.Songs = append(rs.Songs[:i], rs.Songs[i+1:]...)
				if len(rs.Songs) == 0 {
					return b.Delete([]byte(rfid))
				}
				buf, err := json.Marshal(&rs)
				if err != nil {
					return err
				}
				return b.Put([]byte(rfid), buf)
			}
		}
		return nil // songID not in list, idempotent
	})
}

func (s *SongDB) GetSongRFID(songID string) (*model.RFIDSong, error) {
	if songID == "" {
		return nil, fmt.Errorf("songID required")
	}
	var out *model.RFIDSong
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		var found bool
		err := b.ForEach(func(k, v []byte) error {
			if found {
				return nil
			}
			var rs model.RFIDSong
			if err := json.Unmarshal(v, &rs); err != nil {
				return err
			}
			for _, id := range rs.Songs {
				if id == songID {
					out = &rs
					found = true
					return nil
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SongDB) DeleteSongFromRFID(songID string) error {
	if songID == "" {
		return fmt.Errorf("songID required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		var toDelete [][]byte
		var toPut []struct {
			key []byte
			val []byte
		}
		if err := b.ForEach(func(k, v []byte) error {
			var rs model.RFIDSong
			if err := json.Unmarshal(v, &rs); err != nil {
				return err
			}
			for i, id := range rs.Songs {
				if id != songID {
					continue
				}
				rs.Songs = append(rs.Songs[:i], rs.Songs[i+1:]...)
				if len(rs.Songs) == 0 {
					toDelete = append(toDelete, append([]byte(nil), k...))
				} else {
					buf, err := json.Marshal(&rs)
					if err != nil {
						return err
					}
					toPut = append(toPut, struct {
						key []byte
						val []byte
					}{append([]byte(nil), k...), buf})
				}
				return nil
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		for _, p := range toPut {
			if err := b.Put(p.key, p.val); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SongDB) DeleteRFID(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		return b.Delete([]byte(id))
	})
}
