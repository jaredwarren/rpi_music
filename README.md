# Raspberry Pi RFID Music Player

## Setup instructions
## 0. Setup Raspberry Pi
Install Raspbian in headless mode.

## 1. dependances
<!-- `sudo apt install alsa-utils` maybe to control volume -->
Install `ffmpeg` (required)

## 2. generate a self-signed SSL cert (optional)
In order for NFC to work on Android a ssl/https cert is needed. Self-signed works, if you ignore the alert.

from - https://medium.com/rungo/secure-https-servers-in-go-a783008b36da
`openssl req  -new  -newkey rsa:2048  -nodes  -keyout localhost.key  -out localhost.csr`
`openssl  x509  -req  -days 365  -in localhost.csr  -signkey localhost.key  -out localhost.crt`



# TODO #
- [ ] create system for assigning cards without phone
- [ ] volume control!
  - `amixer -D pulse sset Master 5%+`?
- [ ] light or sound to show status
  - `speaker-test -t sine -f 1000 -l 1`
- [ ] keep alive when submitting new song
- [ ] clean/delete logs, view logs in ui
- [ ] automate push build to pi, restart
- [ ] Documentation and cleanup
- [ ] add more to config page, like...
 
## Playlist
- [ ] Playlist - create, edit, update local play list
- [ ] Playlist - download from youtube
- [ ] player queue, add mode to enqueue songs
 
## Case
- [ ] add stl's to repo
- [ ] add USB port to stl to allow power to speaker
 
## Nice to have
- [ ] logs and stats
  - add page to view logs/stats
- [ ] Write url to card
  - wouldn't need a db.
- [ ] push notification to android
  - push to see what's playing, etc.
- [ ] see if I can control led on rfid, or add another led to rpi
 
## Player on phone
- [ ] create player bar on all?
- [ ] re-do ui to be more like yt-music app


# Helpful links

## rpi audio
https://www.raspberrypi-spy.co.uk/2019/06/using-a-usb-audio-device-with-the-raspberry-pi/

## systemd
https://www.dexterindustries.com/howto/run-a-program-on-your-raspberry-pi-at-startup/#systemd

## Icon Library
https://fonts.google.com/icons


### systemd logs
`journalctl -u player.service`