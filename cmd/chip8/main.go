package main

import (
	"log"
	"os"

	"github.com/dnridwn/chip8/internal/emulator"
)

func main() {
	if len(os.Args) < 2 {
		log.Println("usage: (chip8|go run cmd/chip8/main.go) <rom_path>")
		os.Exit(1)
	}

	rom := os.Args[1]
	emulator := emulator.NewEmulator()

	if err := emulator.LoadROM(rom); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	if err := emulator.Run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
