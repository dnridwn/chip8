package emulator

import (
	"errors"
	"fmt"
	"os"
	"time"

	"math/rand"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	memorySize         = 4096
	memoryStartPointer = 0x200
	screenWidth        = 64
	screenHeight       = 32
)

var beepSound []byte

type Emulator struct {
	memory     [memorySize]byte
	display    [screenHeight * screenWidth]byte
	stack      [16]uint16
	v          [16]byte
	pc         uint16
	sp         byte
	i          uint16
	keys       [16]bool
	delayTimer byte
	soundTimer byte
}

func NewEmulator() *Emulator {
	emulator := &Emulator{}
	emulator.pc = memoryStartPointer
	return emulator
}

func (c *Emulator) LoadROM(filename string) error {
	fc, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read rom: %v", err)
	}

	romSize := len(fc)
	maxSize := len(c.memory) - memoryStartPointer
	if romSize > maxSize {
		return fmt.Errorf("rom is too large. maximum: %d, got: %d", maxSize, romSize)
	}

	copy(c.memory[memoryStartPointer:], fc)
	return nil
}

func (c *Emulator) clearDisplay() {
	for i := 0; i < len(c.display); i++ {
		c.display[i] = 0
	}
}

func (c *Emulator) drawSprite(x, y, n byte) {
	posx := int(c.v[x] % 64)
	posy := int(c.v[y] % 32)

	c.v[0xF] = 0

	for row := 0; row < int(n); row++ {
		if posy+row >= 32 {
			break
		}

		spriteByte := c.memory[c.i+uint16(row)]
		for col := 0; col < 8; col++ {
			if posx+col >= 64 {
				break
			}

			pixelIndex := (posy+row)*64 + (posx + col)
			pixelValue := (spriteByte >> (7 - col)) & 1
			if pixelValue == 1 {
				if c.display[pixelIndex] == 1 {
					c.v[0xF] = 1
				}
				c.display[pixelIndex] ^= 1
			}
		}
	}
}

func (c *Emulator) fetchOpcode() uint16 {
	return uint16(c.memory[c.pc])<<8 | uint16(c.memory[c.pc+1])
}

func (c *Emulator) executeOpcode(opcode uint16) error {
	var (
		x   = byte((opcode & 0x0F00) >> 8)
		y   = byte((opcode & 0x00F0) >> 4)
		n   = byte(opcode & 0x000F)
		nn  = byte(opcode & 0x00FF)
		nnn = opcode & 0x0FFF
	)

	switch opcode & 0xF000 {
	case 0x0000:
		switch nn {
		case 0x000E:
			c.clearDisplay()

		case 0x00EE:
			if c.sp == 0 {
				return errors.New("stack underflow. no subroutine to call from")
			}
			c.sp--
			c.pc = c.stack[c.sp]
		}

	case 0x1000:
		c.pc = nnn

	case 0x2000:
		if c.sp > byte(len(c.stack)-1) {
			return errors.New("stack overflow. subroutine call exceeded stack size")
		}
		c.stack[c.sp] = c.pc
		c.sp++
		c.pc = nnn

	case 0x3000:
		if c.v[x] == nn {
			c.pc += 2
		}

	case 0x4000:
		if c.v[x] != nn {
			c.pc += 2
		}

	case 0x5000:
		if c.v[x] == c.v[y] {
			c.pc += 2
		}

	case 0x6000:
		c.v[x] = nn

	case 0x7000:
		c.v[x] += nn

	case 0x8000:
		switch n {
		case 0x0000:
			c.v[x] = c.v[y]

		case 0x0001:
			c.v[x] |= c.v[y]

		case 0x0002:
			c.v[x] &= c.v[y]

		case 0x0003:
			c.v[x] ^= c.v[y]

		case 0x0004:
			c.v[x] = c.v[x] + c.v[y]
			c.v[0xF] = 0
			if c.v[x] < c.v[y] {
				c.v[0xF] = 1
			}

		case 0x0005:
			if c.v[x] >= c.v[y] {
				c.v[0xF] = 1
			} else {
				c.v[0xF] = 0
			}
			c.v[x] = c.v[x] - c.v[y]

		case 0x0006:
			c.v[0xF] = c.v[x] & 0x1
			c.v[x] >>= 1

		case 0x0007:
			if c.v[y] >= c.v[x] {
				c.v[0xF] = 1
			} else {
				c.v[0xF] = 0
			}
			c.v[x] = c.v[y] - c.v[x]

		case 0x000E:
			c.v[0xF] = (c.v[x] & 0x80) >> 7
			c.v[x] <<= 1
		}

	case 0x9000:
		if c.v[x] != c.v[y] {
			c.pc += 2
		}

	case 0xA000:
		c.i = nnn

	case 0xB000:
		c.pc = nnn + uint16(c.v[0])

	case 0xC000:
		randB := byte(rand.Intn(256))
		c.v[x] = randB & nn

	case 0xD000:
		c.drawSprite(x, y, n)

	case 0xE000:
		switch nn {
		case 0x009E:
			if c.v[x] > byte(len(c.keys)-1) {
				return errors.New("stack overflow. trying to access invalid key address")
			}
			if c.keys[c.v[x]] {
				c.pc += 2
			}

		case 0x00A1:
			if c.v[x] > byte(len(c.keys)-1) {
				return errors.New("stack overflow. trying to access invalid key address")
			}
			if !c.keys[c.v[x]] {
				c.pc += 2
			}
		}

	case 0xF000:
		switch nn {
		case 0x07:
			c.v[x] = c.delayTimer

		case 0x0A:
			keyPressed := false
			for i, pressed := range c.keys {
				if pressed {
					c.v[x] = byte(i)
					keyPressed = true
					break
				}
			}
			if !keyPressed {
				c.pc -= 2
			}

		case 0x15:
			c.delayTimer = c.v[x]

		case 0x18:
			c.soundTimer = c.v[x]

		case 0x1E:
			c.i += uint16(c.v[x])

		case 0x29:
			c.i = uint16(c.v[x]) * 5

		case 0x33:
			c.memory[c.i] = c.v[x] / 100
			c.memory[c.i+1] = (c.v[x] / 10) % 10
			c.memory[c.i+2] = c.v[x] % 10

		case 0x55:
			for i := byte(0); i <= x; i++ {
				c.memory[c.i+uint16(i)] = c.v[i]
			}

		case 0x65:
			for i := byte(0); i <= x; i++ {
				c.v[i] = c.memory[c.i+uint16(i)]
			}
		}
	}

	return nil
}

func (c *Emulator) initAudio() error {
	if err := sdl.Init(sdl.INIT_AUDIO); err != nil {
		return fmt.Errorf("failed to init audio: %v", err)
	}

	spec := &sdl.AudioSpec{
		Freq:     44100,
		Format:   sdl.AUDIO_U8,
		Channels: 1,
		Samples:  512,
		Callback: nil,
	}

	if err := sdl.OpenAudio(spec, nil); err != nil {
		return fmt.Errorf("failed to open audio: %v", err)
	}

	beepSound = make([]byte, 44100/30)
	for i := range beepSound {
		if i%2 == 0 {
			beepSound[i] = 255 // High
		} else {
			beepSound[i] = 0 // Low
		}
	}

	return nil
}

func playBeep() {
	sdl.QueueAudio(1, beepSound)
	sdl.PauseAudio(false)
}

func stopBeep() {
	sdl.PauseAudio(true)
}

func (c *Emulator) initGraphics() (*sdl.Renderer, *sdl.Window, error) {
	err := sdl.Init(sdl.INIT_EVERYTHING)
	if err != nil {
		return nil, nil, err
	}

	window, err := sdl.CreateWindow("Chip-8 Emulator", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, screenWidth*10, screenHeight*10, sdl.WINDOW_SHOWN)
	if err != nil {
		return nil, nil, err
	}

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		return nil, nil, err
	}

	return renderer, window, nil
}

func (c *Emulator) render(renderer *sdl.Renderer) {
	renderer.SetDrawColor(0, 0, 0, 255)
	renderer.Clear()
	renderer.SetDrawColor(255, 255, 255, 255)

	for i := 0; i < len(c.display); i++ {
		if c.display[i] == 1 {
			x := (i % screenWidth) * 10
			y := (i / screenWidth) * 10
			rect := sdl.Rect{X: int32(x), Y: int32(y), W: 10, H: 10}
			renderer.FillRect(&rect)
		}
	}

	renderer.Present()
}

func (c *Emulator) startTimers() {
	ticker := time.NewTicker(time.Millisecond * 16)
	defer ticker.Stop()

	for range ticker.C {
		if c.delayTimer > 0 {
			c.delayTimer--
		}

		if c.soundTimer > 0 {
			c.soundTimer--
			playBeep()
		} else {
			stopBeep()
		}
	}
}

func (c *Emulator) Run() error {
	if err := c.initAudio(); err != nil {
		return fmt.Errorf("failed to init audio: %v", err)
	}

	renderer, window, err := c.initGraphics()
	if err != nil {
		return fmt.Errorf("failed to init graphics: %v", err)
	}

	defer window.Destroy()
	defer renderer.Destroy()

	go c.startTimers()

	for {
		opcode := c.fetchOpcode()
		c.pc += 2
		if err := c.executeOpcode(opcode); err != nil {
			return fmt.Errorf("failed to execute opcode: %x: %v", opcode, err)
		}

		c.render(renderer)
		time.Sleep(time.Millisecond * 2)
	}
}
