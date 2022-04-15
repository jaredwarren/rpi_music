



## Setup

### rpi setup
#### default rpi stuff
#### md
song_files?
thumb_files
config/config.yml
#### dependances
<!-- `sudo apt install alsa-utils` maybe -->
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

keep alive

css

add link to "songs" to show player without playing
Documentation and cleanup
Playlist

download image with video

add config page
config 
 - override current playing
  - else queue?
 - repeat?
 - ...




## Case
add stl's


## Nice to have
logs and stats
from phone check existing card, and other card management
mobile friendly ui
create player bar on all?
Write to card url/name????
push notification to android
uninstall pulseaudio
see if I can control led on rfid, or add another led to rpi



# Helpful links

## rpi audio
https://www.raspberrypi-spy.co.uk/2019/06/using-a-usb-audio-device-with-the-raspberry-pi/

## systemd
https://www.dexterindustries.com/howto/run-a-program-on-your-raspberry-pi-at-startup/#systemd
### systemd logs
`journalctl -u service-name`