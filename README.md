



## Setup

### rpi setup
#### default rpi stuff
#### dependances
<!-- `sudo apt install alsa-utils` maybe to control volume -->
`ffmpeg` required


### install

### gen cert
In order for NFC to work on Android a ssl/https cert is needed. Self-signed works, if you ignore the alert.

from - https://medium.com/rungo/secure-https-servers-in-go-a783008b36da
`openssl req  -new  -newkey rsa:2048  -nodes  -keyout localhost.key  -out localhost.csr`
`openssl  x509  -req  -days 365  -in localhost.csr  -signkey localhost.key  -out localhost.crt`






# TODO #
volume!
 - `amixer -D pulse sset Master 5%+`?
light or sound to show status
 - `speaker-test -t sine -f 1000 -l 1`

keep alive when submitting new song

css, not loving spectre

clean/delete logs, view logs in ui

push build to pi, restart

edit page

Documentation and cleanup

add more to config page, clean up

Playlist
player queue

add systemd setup


## Case
add stl's to repo


## Nice to have
logs and stats
from phone check existing card, and other card management
mobile friendly ui
create player bar on all?
Write to card url/name????
push notification to android
uninstall pulseaudio
see if I can control led on rfid, or add another led to rpi

re-do ui to be more like yt-music app



# Helpful links

## rpi audio
https://www.raspberrypi-spy.co.uk/2019/06/using-a-usb-audio-device-with-the-raspberry-pi/

## systemd
https://www.dexterindustries.com/howto/run-a-program-on-your-raspberry-pi-at-startup/#systemd


### systemd logs
`journalctl -u player.service`