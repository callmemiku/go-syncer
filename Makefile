windows:
	GOOS=windows GOARCH=amd64 go build -o build/syncer.exe -ldflags="-s -w" main.go
linux:
	GOOS=linux GOARCH=arm64 go build -o build/syncer -ldflags="-s -w" main.go

