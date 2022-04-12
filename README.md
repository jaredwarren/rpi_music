






## gen cert
In order for NFC to work on Android a ssl/https cert is needed. Self-signed works, if you ignore the alert.

from - https://medium.com/rungo/secure-https-servers-in-go-a783008b36da
`openssl req  -new  -newkey rsa:2048  -nodes  -keyout localhost.key  -out localhost.csr`
`openssl  x509  -req  -days 365  -in localhost.csr  -signkey localhost.key  -out localhost.crt`






# TODO #
add link to "songs" to show player without playing
Autostart
Documentation and cleanup
Playlist

## Case
design and make
(add card holder)

## Nice to have
logs and stats
from phone check existing card, and other card management
mobile friendly ui
create player bar on all?ß
Write to card url/name????
push notification to android
uninstall pulseaudio
see if I can control led on rfid, or add another led to rpi

