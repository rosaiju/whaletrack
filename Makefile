.PHONY: build test lint clean run

build:
	go build -o whaletrack .

test:
	go test ./... -v

lint:
	go vet ./...

coverage:
	go test ./... -coverprofile=coverage.txt -covermode=atomic
	go tool cover -func=coverage.txt

clean:
	rm -f whaletrack whaletrack.exe coverage.txt

run: build
	./whaletrack scan --type purchases --min-value 500000 --days 30
