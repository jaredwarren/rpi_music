package rfid

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	host "periph.io/x/host/v3"
)

// default pins used for rest and irq
const (
	resetPin = "P1_22" // GPIO 25
	irqPin   = "P1_18" // GPIO 24
)

type Config struct {
	SPIPort    string
	ResetPin   string
	IRQPin     string
	IRQTimeout time.Duration
}

type RFIDReader struct {
	RFID       *mfrc522.Dev
	port       spi.PortCloser
	IRQTimeout time.Duration
	IsReady    bool
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
	log.Printf("Setting up new reader:%+v\n", cfg)

	rr := &RFIDReader{}

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

	log.Println("Started rfid reader.")
	return rr, nil
}

func (r *RFIDReader) ReadID() string {
	timedOut := false
	cb := make(chan []byte)
	defer func() {
		timedOut = true
		close(cb)
	}()
	go func() {
		log.Printf("ready %s", r.RFID.String())
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
		log.Printf("[E] rfid.Halt:", err)
	}
	if err := r.port.Close(); err != nil {
		log.Printf("[E] port.Close:", err)
	}
}
