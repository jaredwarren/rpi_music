
build:
	go build -o bin/main .

run:
	./bin/main

fmt:
	gofmt -w .