build:
	go fmt
	go mod tidy
	cd cmd/ortfomk; go build

install:
	chmod +x cmd/ortfomk/ortfomk
	sudo cp cmd/ortfomk/ortfomk ~/.local/bin/ortfomk
