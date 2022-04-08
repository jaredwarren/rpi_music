






## gen cert
In order for NFC to work on Android a ssl/https cert is needed. Self-signed works, if you ignore the alert.
from - https://medium.com/rungo/secure-https-servers-in-go-a783008b36da
`openssl req  -new  -newkey rsa:2048  -nodes  -keyout localhost.key  -out localhost.csr`
`openssl  x509  -req  -days 365  -in localhost.csr  -signkey localhost.key  -out localhost.crt`






TODO:

build controls in browser

logs

from phone check existing card

Write to card url/name????

Autostart

Documentation

uninstall pulseaudio
see if I can control led on rfid

