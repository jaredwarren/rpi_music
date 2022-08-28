

# Used for testing
run: export USERNAME = foo
run: export PASSWORD = bar
run: export LOCALTUNNEL_HOST = asdffdsa

build:
	go build -o pplayer .

test: 
	go test ./... 

run:
	go run main.go

fmt:
	gofmt -w .

pi-build:
	GOARM=6 GOARCH=arm GOOS=linux go build -o pplayer .

# push doesn't work for some reason
# need to push templates too!
pi-push:
	scp -r pplayer pi@192.168.1.178:/home/pi/go/src/github.com/jaredwarren/rpi_music/pplayer

