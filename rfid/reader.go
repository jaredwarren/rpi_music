package rfid

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jaredwarren/rpi_music/db"
	"github.com/jaredwarren/rpi_music/log"
	"github.com/jaredwarren/rpi_music/player"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	host "periph.io/x/host/v3"
)

// default pins used for rest and irq
const (
	// TODO: move to config
	resetPin = "P1_22" // GPIO 25
	irqPin   = "P1_18" // GPIO 24
)

func InitRFIDReader(db db.DBer, logger log.Logger) *RFIDReader {
	logger.Info("Initializing RFID")
	cfg := DefaultConfig()
	cfg.logger = logger
	cfg.db = db
	r, err := New(cfg)
	if err != nil {
		logger.Panic("error creating new reader", log.Error(err))
	}

	r.Start()
	return r
}

type Config struct {
	SPIPort    string
	ResetPin   string
	IRQPin     string
	IRQTimeout time.Duration
	logger     log.Logger
	db         db.DBer
}

type RFIDReader struct {
	RFID       *mfrc522.Dev
	port       spi.PortCloser
	IRQTimeout time.Duration
	IsReady    bool
	logger     log.Logger
	db         db.DBer
}

func DefaultConfig() *Config {
	cfg := &Config{}
	// Note: SPIPort is ""
	if cfg.ResetPin == "" {
		cfg.ResetPin = resetPin
	}
	if cfg.IRQPin == "" {
		cfg.IRQPin = irqPin
	}
	return cfg
}

/*
Setup inits and starts hardware.
*/
func New(cfg *Config) (*RFIDReader, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	rr := &RFIDReader{
		logger: cfg.logger,
		db:     cfg.db,
	}

	// guarantees all drivers are loaded.
	if _, err := host.Init(); err != nil {
		return nil, err
	}

	// get the first available spi port eith empty string.
	port, err := spireg.Open(cfg.SPIPort)
	if err != nil {
		return nil, err
	}
	rr.port = port

	// get GPIO rest pin from its name
	var gpioResetPin gpio.PinOut = gpioreg.ByName(cfg.ResetPin)
	if gpioResetPin == nil {
		return nil, fmt.Errorf("Failed to find %v", cfg.ResetPin)
	}

	// get GPIO irq pin from its name
	var gpioIRQPin gpio.PinIn = gpioreg.ByName(cfg.IRQPin)
	if gpioIRQPin == nil {
		return nil, fmt.Errorf("Failed to find %v", cfg.IRQPin)
	}

	rfid, err := mfrc522.NewSPI(rr.port, gpioResetPin, gpioIRQPin, mfrc522.WithSync())
	if err != nil {
		return nil, err
	}
	rr.RFID = rfid

	// setting the antenna signal strength, signal strength from 0 to 7
	rr.RFID.SetAntennaGain(5)

	rr.IsReady = true

	cfg.logger.Info("RFID ready")
	return rr, nil
}

func (r *RFIDReader) Start() {
	go func() {
		for {
			rfid := r.ReadID()
			rfidSong, err := r.db.GetRFIDSong(rfid)
			if err != nil {
				r.logger.Error("GetRFIDSong error", log.Error(err))
				// TODO: play error
				continue
			}
			if len(rfidSong.Songs) == 0 {
				r.logger.Error("no songs"+rfid, log.Error(err))
				// TODO: play error
				continue
			}

			song, err := r.db.GetSongV2(rfidSong.Songs[0])
			if err != nil {
				r.logger.Error("error reading db", log.Error(err))
			}
			if song != nil {
				r.logger.Info("found song", log.Any("song", song))
				player.Beep()
				err := player.Play(song)
				if err != nil {
					r.logger.Error("error playing song", log.Error(err))
				}
			} else {
				r.logger.Info("song id not found", log.Any("id", rfid))
			}

			// cooldown
			time.Sleep(2 * time.Second)
		}
	}()
}

func (r *RFIDReader) ReadID() string {
	timedOut := false
	cb := make(chan []byte)
	defer func() {
		timedOut = true
		close(cb)
	}()
	go func() {
		for {
			// trying to read UID
			if r.IsReady {
				data, err := r.RFID.ReadUID(5 * time.Second) // Note: timeout is IRQ timeout
				// Prevent trying to write to closed channel
				if timedOut {
					return
				}
				// Some devices tend to send wrong data while RFID chip is already detected
				// but still "too far" from a receiver.
				// Especially some cheap CN clones which you can find on GearBest, AliExpress, etc.
				// This will suppress such errors.
				if err != nil {
					continue
				}
				cb <- data
			} // else return ?????
			time.Sleep(100 * time.Millisecond) // added some delay.
		}
	}()

	data := <-cb
	return hex.EncodeToString(data)
}

func (r *RFIDReader) Close() {
	r.IsReady = false
	// closerfid is idling the RFID device and closes spi port.
	if err := r.RFID.Halt(); err != nil {
		r.logger.Error("rfid halt error", log.Error(err))
	}
	if err := r.port.Close(); err != nil {
		r.logger.Error("rfid port close error", log.Error(err))
	}
}
