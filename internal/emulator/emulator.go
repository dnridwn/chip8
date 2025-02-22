package emulator

import (
	"errors"
	"fmt"
	"os"
	"time"

	"math/rand"

	"github.com/veandco/go-sdl2/mix"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	memorySize         = 4096
	memoryStartPointer = 0x200
	screenWidth        = 64
	screenHeight       = 32
)

var (
	fontSet = []byte{
		0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
		0x20, 0x60, 0x20, 0x20, 0x70, // 1
		0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
		0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
		0x90, 0x90, 0xF0, 0x10, 0x10, // 4
		0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
		0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
		0xF0, 0x10, 0x20, 0x40, 0x40, // 7
		0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
		0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
		0xF0, 0x90, 0xF0, 0x90, 0x90, // A
		0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
		0xF0, 0x80, 0x80, 0x80, 0xF0, // C
		0xE0, 0x90, 0x90, 0x90, 0xE0, // D
		0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
		0xF0, 0x80, 0xF0, 0x80, 0x80, // F
	}
)

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
	beepSound  *mix.Chunk
}

func NewEmulator() *Emulator {
	emulator := &Emulator{}
	emulator.pc = memoryStartPointer
	emulator.loadFontSet()
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

func (c *Emulator) loadFontSet() {
	for i := 0; i < 80; i++ {
		c.memory[i] = fontSet[i]
	}
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
					c.keys[i] = false
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
	if err := mix.OpenAudio(44100, mix.DEFAULT_FORMAT, 1, 2048); err != nil {
		return fmt.Errorf("failed to open audio: %v", err)
	}

	var err error
	c.beepSound, err = mix.LoadWAV("sounds/beep.wav")
	if err != nil {
		return fmt.Errorf("failed to open beep.wav: %v", err)
	}

	return nil
}

func (c *Emulator) playBeep() {
	c.beepSound.Play(-1, 0)
}

func (c *Emulator) stopBeep() {
	mix.HaltChannel(-1)
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

func (c *Emulator) startTimers(quit chan struct{}) {
	ticker := time.NewTicker(time.Millisecond * 16)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.delayTimer > 0 {
				c.delayTimer--
			}

			if c.soundTimer > 0 {
				c.soundTimer--
				c.playBeep()
			} else {
				c.stopBeep()
			}
		case <-quit:
			return
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

	quit := make(chan struct{})
	go c.startTimers(quit)

	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch e := event.(type) {
			case *sdl.KeyboardEvent:
				switch event.GetType() {
				case sdl.KEYUP:
					switch e.Keysym.Sym {
					case '1':
						c.keys[0x1] = true
					case '2':
						c.keys[0x2] = true
					case '3':
						c.keys[0x3] = true
					case '4':
						c.keys[0xC] = true
					case 'q':
						c.keys[0x4] = true
					case 'w':
						c.keys[0x5] = true
					case 'e':
						c.keys[0x6] = true
					case 'r':
						c.keys[0xD] = true
					case 'a':
						c.keys[0x7] = true
					case 's':
						c.keys[0x8] = true
					case 'd':
						c.keys[0x9] = true
					case 'f':
						c.keys[0xE] = true
					case 'z':
						c.keys[0xA] = true
					case 'x':
						c.keys[0x0] = true
					case 'c':
						c.keys[0xB] = true
					case 'v':
						c.keys[0xF] = true
					}
				}
			case *sdl.QuitEvent:
				close(quit)
				return nil
			}
		}

		opcode := c.fetchOpcode()
		c.pc += 2
		if err := c.executeOpcode(opcode); err != nil {
			return fmt.Errorf("failed to execute opcode: %x: %v", opcode, err)
		}

		c.render(renderer)
		time.Sleep(time.Millisecond * 2)
	}
}
