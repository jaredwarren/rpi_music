
build:
	go build -o pplayer .

run:
	./pplayer

fmt:
	gofmt -w .

pi-build:
	GOARM=6 GOARCH=arm GOOS=linux go build -o pplayer .

pi-push:
	scp -r pplayer pi@192.168.1.178:/home/pi/go/src/github.com/jaredwarren/rpi_music/pplayer