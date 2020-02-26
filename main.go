package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gdamore/tcell"
)

type keypad struct {
	keys map[uint8]bool
	mux  sync.Mutex
}

type keymap map[rune]uint8

type machine struct {
	instruction    uint16
	memory         [4096]uint8
	registers      [16]uint8
	indexRegister  uint16
	programCounter uint16
	display        [64 * 32]bool
	delayTimer     uint8
	soundTimer     uint8
	stack          [16]uint16
	stackPointer   uint16
	drawFlag       bool
	running        bool
	debugString    string
	keypad         *keypad
}

type font [80]byte

func getDefaultFont() font {
	return font{
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
}

func setupScreen() tcell.Screen {
	var err error
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	s.Clear()

	return s
}

func setupInput(s tcell.Screen, m *machine, km keymap) {
	go func() {
		for m.running {
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:

				k := ev.Key()
				r := ev.Rune()

				if mapped, ok := km[r]; ok {
					m.debugString = fmt.Sprintf("%x", mapped)
					m.pressKey(mapped)
				}

				switch k {
				case tcell.KeyEsc, tcell.KeyCtrlZ, tcell.KeyCtrlC:
					m.running = false
				}

			}
		}
	}()
}

func newKeymap() keymap {
	return keymap{
		'1': 0x1,
		'2': 0x2,
		'3': 0x3,
		'4': 0xC,
		'q': 0x4,
		'w': 0x5,
		'e': 0x6,
		'r': 0xE,
		'a': 0x7,
		's': 0x8,
		'd': 0x9,
		'f': 0xE,
		'z': 0xA,
		'x': 0x0,
		'c': 0xB,
		'v': 0xF,
	}
}

func newKeypad() *keypad {
	return &keypad{keys: make(map[uint8]bool)}
}

func (kp *keypad) press(k uint8) {
	kp.mux.Lock()
	kp.keys[k] = true
	kp.mux.Unlock()
}

func (kp *keypad) state(k uint8) bool {
	s := false
	kp.mux.Lock()
	s = kp.keys[k]
	kp.mux.Unlock()
	return s
}

func (kp *keypad) release(k uint8) {
	kp.mux.Lock()
	kp.keys[k] = false
	kp.mux.Unlock()
}

func (m *machine) clearDisplay() {
	for i := 0; i < cap(m.display); i++ {
		m.display[i] = false
	}
}

func (m *machine) clearStack() {
	for i := 0; i < cap(m.stack); i++ {
		m.stack[i] = 0
	}
}

func (m *machine) clearRegisters() {
	for i := 0; i < cap(m.registers); i++ {
		m.registers[i] = 0
	}
}

func (m *machine) clearMemory() {
	for i := 0; i < cap(m.memory); i++ {
		m.memory[i] = 0
	}
}

func (m *machine) loadFont(f font) {
	for i := 0; i < cap(f); i++ {
		m.memory[i] = f[i]
	}
}

func (m *machine) clearKeypad() {
	m.keypad.clear()
}

func (kp *keypad) clear() {
	kp.mux.Lock()

	for k, _ := range kp.keys {
		kp.keys[k] = false
	}

	kp.mux.Unlock()
}

func (m *machine) pressKey(key uint8) {
	m.keypad.press(key)
}

func (m *machine) init(f font) {
	m.programCounter = 0x200
	m.instruction = 0
	m.indexRegister = 0
	m.stackPointer = 0
	m.drawFlag = false
	m.running = true
	m.clearDisplay()
	m.clearStack()
	m.clearRegisters()
	m.clearMemory()
	m.loadFont(f)
	m.clearKeypad()
}

func newMachine(f font) *machine {
	m := &machine{keypad: newKeypad()}
	m.init(f)
	return m
}

func (m *machine) loadRom(path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	memSlice := m.memory[512:]
	_, err = file.Read(memSlice)
	if err != nil {
		log.Fatal(err)
	}

	file.Close()
}

type renderer func(m *machine)

func (m *machine) render(render renderer) {
	if m.drawFlag {
		render(m)
		m.drawFlag = false
	}
}

func (m *machine) run(r renderer) {
	for m.running {
		m.processInstruction()
		m.render(r)
		m.updateDelayTimer()
		m.updateSoundTimer()
		time.Sleep(1 * time.Millisecond)
	}
}

func (m *machine) updateDelayTimer() {
	if m.delayTimer > 0 {
		m.delayTimer--
	}
}

func (m *machine) updateSoundTimer() {
	if m.soundTimer > 0 {
		if m.soundTimer == 1 {

		}
		m.soundTimer--
	}
}

func render(s tcell.Screen) func(m *machine) {
	return func(m *machine) {
		s.Clear()
		style := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorBlack)

		for y := 0; y < 32; y++ {
			for x := 0; x < 64; x++ {
				if m.display[x+y*64] {
					s.SetContent(x, y, '*', nil, style)
				} else {
					s.SetContent(x, y, ' ', nil, style)
				}
			}
		}

		debugStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack)

		for i, c := range m.debugString {
			s.SetContent(20+i, 25, c, nil, debugStyle)
		}

		s.Show()
	}
}

func main() {

	f := getDefaultFont()
	s := setupScreen()
	km := newKeymap()
	m := newMachine(f)
	setupInput(s, m, km)

	m.loadRom("ROMS/TANK")

	m.run(render(s))

	s.Fini()
}

func (m *machine) processInstruction() {

	//Fetch instruction
	// m.memory is 4096 bytes, instruction is 2 bytes, so we pull two addresses and combine them
	// Shift first to the left 8bits, or it with the next
	m.instruction = uint16(m.memory[m.programCounter])<<8 | uint16(m.memory[m.programCounter+1])

	switch m.instruction & 0xF000 {

	case 0x0000:
		switch m.instruction {
		// 00E0 - Clears the screen
		case 0x00E0:
			// Clear display
			for i := 0; i < cap(m.display); i++ {
				m.display[i] = false
			}
			m.programCounter += 2
			m.drawFlag = true
			return
		// 00EE - Returns from a subroutine.
		case 0x00EE:
			m.stackPointer--
			m.programCounter = m.stack[m.stackPointer] + 2
			return
		// Calls RCA 1802 program at address NNN. Not necessary for most ROMs. skipping impl. but could use 2NNN i think....
		default:
		}

	// 1NNN - Jumps to address NNN.
	case 0x1000:
		m.programCounter = m.instruction & 0x0FFF
		return
	// 2NNN - Calls subroutine at NNN
	case 0x2000:
		//First store the current prog counter in the stack so we can track it later
		m.stack[m.stackPointer] = m.programCounter
		//Bump the stack pointer (same thing as we do with prog counter)
		m.stackPointer++
		//set program counter to point to the subroutine
		m.programCounter = m.instruction & 0x0FFF
		//^ Assume when the subroutine flow finishes, we pop the stack onto the prog counter and continue
		return

	// 3XNN	- Skips the next instruction if VX equals NN. (Usually the next instruction is a jump to skip a code block)
	case 0x3000:
		x := m.instruction & 0x0F00 >> 8
		n := uint8(m.instruction & 0x00FF)

		if m.registers[x] == n {
			m.programCounter += 4
		} else {
			m.programCounter += 2
		}

		return
	// 4XNN	Skips the next instruction if VX doesn't equal NN. (Usually the next instruction is a jump to skip a code block)
	case 0x4000: // 4XNN
		x := m.instruction & 0x0F00 >> 8
		n := uint8(m.instruction & 0x00FF)

		if m.registers[x] != n {
			m.programCounter += 4
		} else {
			m.programCounter += 2
		}
		return

	// 5XY0	Skips the next instruction if VX equals VY. (Usually the next instruction is a jump to skip a code block)
	case 0x5000: // 5XY0
		x := m.instruction & 0x0F00 >> 8
		y := m.instruction & 0x00F0 >> 4

		if m.registers[x] == m.registers[y] {
			m.programCounter += 4
		} else {
			m.programCounter += 2
		}

		return

	// 6XNN - Sets VX to NN
	case 0x6000:
		//extract x (shift to get true value)
		x := m.instruction & 0x0F00 >> 8
		//extract NN (cast to 8 bits/1 byte)
		n := uint8(m.instruction & 0x00FF)
		//Update v[x] with n
		m.registers[x] = n
		//bump prog counter
		m.programCounter += 2
		return

	// 7XNN - Adds NN to VX. (Carry flag is not changed, no overflow check)
	case 0x7000:
		x := m.instruction & 0x0F00 >> 8
		n := m.instruction & 0x00FF

		m.registers[x] += uint8(n)
		m.programCounter += 2
		return

	case 0x8000:
		switch m.instruction & 0xF00F {

		// 8XY0 - Sets VX to the value of VY.
		case 0x8000:
			x := m.instruction & 0x0F00 >> 8
			y := m.instruction & 0x00F0 >> 4

			m.registers[x] = m.registers[y]

			m.programCounter += 2

			return

		// 8XY1 - Sets VX to VX or VY. (Bitwise OR operation)
		case 0x8001:
			x := m.instruction & 0x0F00 >> 8
			y := m.instruction & 0x00F0 >> 4

			m.registers[x] = m.registers[x] | m.registers[y]

			m.programCounter += 2

			return

		// 8XY2 - Sets VX to VX and VY. (Bitwise AND operation)
		case 0x8002:
			x := m.instruction & 0x0F00 >> 8
			y := m.instruction & 0x00F0 >> 4

			m.registers[x] = m.registers[x] & m.registers[y]

			m.programCounter += 2

			return
		// 8XY3 Sets VX to VX xor VY.
		case 0x8003:
			x := m.instruction & 0x0F00 >> 8
			y := m.instruction & 0x00F0 >> 4

			m.registers[x] = m.registers[x] ^ m.registers[y]

			m.programCounter += 2

			return

		// 8XY4 - adds VY to VX, if overflow byte, set VF to 1, otherwise 0
		case 0x8004:

			//Extract args
			x := m.instruction & 0x0F00 >> 8
			y := m.instruction & 0x00F0 >> 4

			sum := uint16(m.registers[x] + m.registers[y])

			//if overflow set the carry flag
			if sum > 255 {
				m.registers[15] = 1
			} else {
				m.registers[15] = 0
			}

			m.registers[x] += m.registers[y]
			m.programCounter += 2

			return

		// 8XY5 - VY is subtracted from VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
		// Set Vx = Vx - Vy, set VF = NOT borrow.
		// If Vx > Vy, then VF is set to 1, otherwise 0. Then Vy is subtracted from Vx, and the results stored in Vx.
		case 0x8005:

			//Extract args
			x := m.instruction & 0x0F00 >> 8
			y := m.instruction & 0x00F0 >> 4

			//if overflow set the carry flag
			if m.registers[x] > m.registers[y] {
				m.registers[15] = 1
			} else {
				m.registers[15] = 0
			}

			m.registers[x] -= m.registers[y]
			m.programCounter += 2

			return

		// 8XY6 - Stores the least significant bit of VX in VF and then shifts VX to the right by 1.
		case 0x8006:

			//Extract args
			x := m.instruction & 0x0F00 >> 8

			// And 1 with our number, e.g number = 01010101 & 00000001 = 1
			lsb := m.registers[x] & 1

			m.registers[0xF] = lsb
			m.registers[x] = m.registers[x] >> 1

			m.programCounter += 2
			return

		// 8XY7 - Sets VX to VY minus VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
		case 0x8007:
			//Extract args
			x := m.instruction & 0x0F00 >> 8
			y := m.instruction & 0x00F0 >> 4

			//if overflow set the carry flag
			if m.registers[y] > m.registers[x] {
				m.registers[15] = 1
			} else {
				m.registers[15] = 0
			}

			m.registers[x] = m.registers[y] - m.registers[x]
			m.programCounter += 2
			return
		// 8XYE - Stores the most significant bit of VX in VF and then shifts VX to the left by 1.
		case 0x800E:

			//Extract args
			x := m.instruction & 0x0F00 >> 8

			// And 1 with our number, e.g number = 01010101 & 10000000 = 1 - this is probably not right... im tired though
			// could try shifting bits completely to find it e.g shift 7 either way
			b := m.registers[x] & 0b10000000

			m.registers[0xF] = b
			m.registers[x] = m.registers[x] << 1

			m.programCounter += 2
			return
		}

	// 9XY0 - Skips the next instruction if VX doesn't equal VY. (Usually the next instruction is a jump to skip a code block)
	case 0x9000:
		x := m.instruction & 0x0F00 >> 8
		y := m.instruction & 0x00F0 >> 4

		if m.registers[x] != m.registers[y] {
			m.programCounter += 4
		} else {
			m.programCounter += 2
		}

		return

	// ANNN - Sets m.indexRegister to NNN
	case 0xA000:
		m.indexRegister = m.instruction & 0x0FFF
		m.programCounter += 2
		return

	// BNNN - Jumps to the address NNN plus V0.
	case 0xB000:
		n := m.instruction & 0x0FFF
		m.programCounter = uint16(m.registers[0] + uint8(n))
		return

	// CXNN - Sets VX to the result of a bitwise and operation on a random number (Typically: 0 to 255) and NN.
	case 0xC000:
		x := m.instruction & 0x0F00 >> 8
		n := m.instruction & 0x00FF
		rand := uint8(12)
		m.registers[x] = uint8(n) & rand
		m.programCounter += 2
		return

	// DXYN - Draws a sprite at coordinate (VX, VY) that has a width of 8 pixels and a height of N pixels.
	// Each row of 8 pixels is read as bit-coded starting from m.memory location I;
	// I value doesn’t change after the execution of this instruction. As described above,
	// VF is set to 1 if any screen pixels are flipped from set to unset when the sprite is drawn, and to 0 if that doesn’t happen
	case 0xD000:
		x := m.instruction & 0x0F00 >> 8
		y := m.instruction & 0x00F0 >> 4
		rows := m.instruction & 0x000F

		xCoord := uint16(m.registers[x])
		yCoord := uint16(m.registers[y])

		m.registers[15] = 0

		for row := uint16(0); row < rows; row++ {

			spriteRow := m.memory[m.indexRegister+row]
			for i := 0; i < 8; i++ {
				bit := spriteRow & (0x80 >> i)
				pos := uint16(xCoord + uint16(i) + 64*(yCoord+row))

				if bit != 0 {
					if m.display[pos] {
						m.registers[15] = 1
					}
					m.display[pos] = !m.display[pos] && true
				}
			}
		}

		m.drawFlag = true
		m.programCounter += 2

		return

	case 0xE000:
		switch m.instruction & 0xF0FF {
		// EX9E - Skips the next instruction if the key stored in VX is pressed. (Usually the next instruction is a jump to skip a code block)
		case 0xE09E:
			x := m.instruction & 0x0F00 >> 8
			k := m.registers[x]

			if m.keypad.state(k) {
				m.keypad.release(k)
				m.programCounter += 4
			} else {
				m.programCounter += 2
			}

			return
		// EXA1 - Skips the next instruction if the key stored in VX isn't pressed. (Usually the next instruction is a jump to skip a code block)
		case 0xE0A1:
			x := m.instruction & 0x0F00 >> 8
			k := m.registers[x]
			if !m.keypad.state(k) {
				m.programCounter += 4
			} else {
				m.keypad.release(k)
				m.programCounter += 2
			}
			return
		}
	case 0xF000:
		switch m.instruction & 0xF0FF {

		// FX07 - Sets VX to the value of the delay timer.
		case 0xF007:
			x := m.instruction & 0x0F00 >> 8
			m.registers[x] = m.delayTimer
			m.programCounter += 2
			return

		// FX0A - A key press is awaited, and then stored in VX. (Blocking Operation. All instruction halted until next key event)
		case 0xF00A:
			m.programCounter += 2
			return

		// FX15 - Sets the delay timer to VX.
		case 0xF015:
			x := m.instruction & 0x0F00 >> 8
			m.delayTimer = m.registers[x]
			m.programCounter += 2
			return

		// FX18 - Sets the sound timer to VX.
		case 0xF018: // FX18
			x := m.instruction & 0x0F00 >> 8
			m.soundTimer = m.registers[x]
			m.programCounter += 2
			return

		// FX1E - Adds VX to I. VF is set to 1 when there is a range overflow (I+VX>0xFFF), and to 0 when there isn't.
		case 0xF01E: // FX1E

			x := m.instruction & 0x0F00 >> 8

			sum := uint16(m.indexRegister + uint16(m.registers[x]))

			if sum > 255 {
				m.registers[0xF] = 1
			} else {
				m.registers[0xF] = 0
			}

			m.indexRegister += uint16(m.registers[x])

			m.programCounter += 2
			return

		// FX29 - Sets I to the location of the sprite for the character in VX.
		// Characters 0-F (in hexadecimal) are represented by a 4x5 font.
		case 0xF029: // FX29
			x := uint8(m.instruction & 0x0F00 >> 8)
			m.indexRegister = uint16(m.registers[x] * 5)
			m.programCounter += 2
			return
		// FX33 - Stores the binary-coded decimal representation of VX, with the most significant of three digits at the address in I,
		// the middle digit at I plus 1, and the least significant digit at I plus 2. (In other words, take the decimal representation of VX,
		// place the hundreds digit in m.memory at location in I, the tens digit at location I+1, and the ones digit at location I+2.)
		case 0xF033:
			x := m.instruction & 0x0F00 >> 8
			d := m.registers[x]
			a := uint8(d / 100)
			b := uint8((d - a*100) / 10)
			c := uint8(d - a*100 - b*10)

			m.memory[m.indexRegister] = a
			m.memory[m.indexRegister+1] = b
			m.memory[m.indexRegister+2] = c

			m.programCounter += 2
			return

		// FX55 - Stores V0 to VX (including VX) in m.memory starting at address I.
		// The offset from I is increased by 1 for each value written, but I itself is left unmodified
		case 0xF055:
			x := uint16(m.instruction & 0x0F00 >> 8)
			for i := uint16(0); i <= x; i++ {
				m.memory[m.indexRegister+i] = m.registers[i]
			}
			m.programCounter += 2
			return

		// FX65 - Fills V0 to VX (including VX) with values from m.memory starting at address I.
		// The offset from I is increased by 1 for each value written, but I itself is left unmodified.
		case 0xF065:
			x := uint16(m.instruction & 0x0F00 >> 8)
			for i := uint16(0); i <= x; i++ {
				m.registers[i] = m.memory[m.indexRegister+i]
			}
			m.programCounter += 2
			return
		}
	}

	panic(fmt.Sprintf("Unsupported instruction: %X", m.instruction))
}
