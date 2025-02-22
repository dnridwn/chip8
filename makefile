run:
	export CGO_ENABLED=1 && go run cmd/chip8/main.go $(rom)

build:
	go build -o build/chip8 cmd/chip8/main.go
