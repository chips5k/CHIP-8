package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nsf/termbox-go"
)

var (
	opcode         uint16
	memory         [4096]uint8
	registers      [16]uint8
	indexRegister  uint16
	programCounter uint16
	display        [64 * 32]bool
	delayTimer     uint8
	soundTimer     uint8
	stack          [16]uint16
	stackPointer   uint16
	keys           map[rune]bool
	ticks          int
	drawFlag       bool
	kbChannel      = make(chan int)
)

var fontset = [80]byte{
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

func ms() int64 {
	return (time.Now()).UnixNano() / 1000000
}

func keyboardListener(ch chan int) {
	termbox.SetInputMode(termbox.InputEsc)

	for {
		ex := termbox.PollEvent()
		fmt.Println(rune(ex.Key))

		switch e := termbox.PollEvent(); e.Type {
		case termbox.EventKey:
			switch e.Key {
			case termbox.Key('w'):
				fmt.Println("PUSHED")
				keys['w'] = true
				termbox.Close()
			}

		case termbox.EventError:
			panic(e.Err)
		}
	}
}

func main() {

	if err := termbox.Init(); err != nil {
		panic(err)
	}

	go keyboardListener(kbChannel)

	defer termbox.Close()

	setupGraphics()
	setupInput()

	initialize()
	load("ROMS/Pong")

	running := true

	for running == true {

		emulate()

		// If draw flag set
		if drawFlag {
			render()
			drawFlag = false
		}

		time.Sleep(1 * time.Millisecond)
		handleInput()
		ticks++
	}
}

func processOpcode() {

	//Fetch opcode
	// memory is 4096 bytes, opcode is 2 bytes, so we pull two addresses and combine them
	// Shift first to the left 8bits, or it with the next
	opcode = uint16(memory[programCounter])<<8 | uint16(memory[programCounter+1])

	switch opcode & 0xF000 {

	case 0x0000:
		switch opcode {
		// 00E0 - Clears the screen
		case 0x00E0:
			// Clear display
			for i := 0; i < cap(display); i++ {
				display[i] = false
			}
			programCounter += 2
			drawFlag = true
			return
		// 00EE - Returns from a subroutine.
		case 0x00EE:
			stackPointer--
			programCounter = stack[stackPointer] + 2
			return
		// Calls RCA 1802 program at address NNN. Not necessary for most ROMs. skipping impl. but could use 2NNN i think....
		default:
		}

	// 1NNN - Jumps to address NNN.
	case 0x1000:
		programCounter = opcode & 0x0FFF
		return
	// 2NNN - Calls subroutine at NNN
	case 0x2000:
		//First store the current prog counter in the stack so we can track it later
		stack[stackPointer] = programCounter
		//Bump the stack pointer (same thing as we do with prog counter)
		stackPointer++
		//set program counter to point to the subroutine
		programCounter = opcode & 0x0FFF
		//^ Assume when the subroutine flow finishes, we pop the stack onto the prog counter and continue
		return

	// 3XNN	- Skips the next instruction if VX equals NN. (Usually the next instruction is a jump to skip a code block)
	case 0x3000:
		x := opcode & 0x0F00 >> 8
		n := uint8(opcode & 0x00FF)

		if registers[x] == n {
			programCounter += 4
		} else {
			programCounter += 2
		}

		return
	// 4XNN	Skips the next instruction if VX doesn't equal NN. (Usually the next instruction is a jump to skip a code block)
	case 0x4000: // 4XNN
		x := opcode & 0x0F00 >> 8
		n := uint8(opcode & 0x00FF)

		if registers[x] != n {
			programCounter += 4
		} else {
			programCounter += 2
		}
		return

	// 5XY0	Skips the next instruction if VX equals VY. (Usually the next instruction is a jump to skip a code block)
	case 0x5000: // 5XY0
		x := opcode & 0x0F00 >> 8
		y := opcode & 0x00F0 >> 4

		if registers[x] == registers[y] {
			programCounter += 4
		} else {
			programCounter += 2
		}

		return

	// 6XNN - Sets VX to NN
	case 0x6000:
		//extract x (shift to get true value)
		x := opcode & 0x0F00 >> 8
		//extract NN (cast to 8 bits/1 byte)
		n := uint8(opcode & 0x00FF)
		//Update v[x] with n
		registers[x] = n
		//bump prog counter
		programCounter += 2
		return

	// 7XNN - Adds NN to VX. (Carry flag is not changed, no overflow check)
	case 0x7000:
		x := opcode & 0x0F00 >> 8
		n := opcode & 0x00FF

		registers[x] += uint8(n)
		programCounter += 2
		return

	case 0x8000:
		switch opcode & 0xF00F {

		// 8XY0 - Sets VX to the value of VY.
		case 0x8000:
			x := opcode & 0x0F00 >> 8
			y := opcode & 0x00F0 >> 4

			registers[x] = registers[y]

			programCounter += 2

			return

		// 8XY1 - Sets VX to VX or VY. (Bitwise OR operation)
		case 0x8001:
			x := opcode & 0x0F00 >> 8
			y := opcode & 0x00F0 >> 4

			registers[x] = registers[x] | registers[y]

			programCounter += 2

			return

		// 8XY2 - Sets VX to VX and VY. (Bitwise AND operation)
		case 0x8002:
			x := opcode & 0x0F00 >> 8
			y := opcode & 0x00F0 >> 4

			registers[x] = registers[x] & registers[y]

			programCounter += 2

			return
		// 8XY3 Sets VX to VX xor VY.
		case 0x8003:
			x := opcode & 0x0F00 >> 8
			y := opcode & 0x00F0 >> 4

			registers[x] = registers[x] ^ registers[y]

			programCounter += 2

			return

		// 8XY4 - adds VY to VX, if overflow byte, set VF to 1, otherwise 0
		case 0x8004:

			//Extract args
			x := opcode & 0x0F00 >> 8
			y := opcode & 0x00F0 >> 4

			sum := uint16(registers[x] + registers[y])

			//if overflow set the carry flag
			if sum > 255 {
				registers[15] = 1
			} else {
				registers[15] = 0
			}

			registers[x] += registers[y]
			programCounter += 2

			return

		// 8XY5 - VY is subtracted from VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
		// Set Vx = Vx - Vy, set VF = NOT borrow.
		// If Vx > Vy, then VF is set to 1, otherwise 0. Then Vy is subtracted from Vx, and the results stored in Vx.
		case 0x8005:

			//Extract args
			x := opcode & 0x0F00 >> 8
			y := opcode & 0x00F0 >> 4

			//if overflow set the carry flag
			if registers[x] > registers[y] {
				registers[15] = 1
			} else {
				registers[15] = 0
			}

			registers[x] -= registers[y]
			programCounter += 2

			return

		// 8XY6 - Stores the least significant bit of VX in VF and then shifts VX to the right by 1.
		case 0x8006:

			//Extract args
			x := opcode & 0x0F00 >> 8

			// And 1 with our number, e.g number = 01010101 & 00000001 = 1
			lsb := registers[x] & 1

			registers[0xF] = lsb
			registers[x] = registers[x] >> 1

			programCounter += 2
			return

		// 8XY7 - Sets VX to VY minus VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
		case 0x8007:
			//Extract args
			x := opcode & 0x0F00 >> 8
			y := opcode & 0x00F0 >> 4

			//if overflow set the carry flag
			if registers[y] > registers[x] {
				registers[15] = 1
			} else {
				registers[15] = 0
			}

			registers[x] = registers[y] - registers[x]
			programCounter += 2
			return
		// 8XYE - Stores the most significant bit of VX in VF and then shifts VX to the left by 1.
		case 0x800E:

			//Extract args
			x := opcode & 0x0F00 >> 8

			// And 1 with our number, e.g number = 01010101 & 10000000 = 1 - this is probably not right... im tired though
			// could try shifting bits completely to find it e.g shift 7 either way
			b := registers[x] & 0b10000000

			registers[0xF] = b
			registers[x] = registers[x] << 1

			programCounter += 2
			return
		}

	// 9XY0 - Skips the next instruction if VX doesn't equal VY. (Usually the next instruction is a jump to skip a code block)
	case 0x9000:
		x := opcode & 0x0F00 >> 8
		y := opcode & 0x00F0 >> 4

		if registers[x] != registers[y] {
			programCounter += 4
		} else {
			programCounter += 2
		}

		return

	// ANNN - Sets indexRegister to NNN
	case 0xA000:
		indexRegister = opcode & 0x0FFF
		programCounter += 2
		return

	// BNNN - Jumps to the address NNN plus V0.
	case 0xB000:
		n := opcode & 0x0FFF
		programCounter = uint16(registers[0] + uint8(n))
		return

	// CXNN - Sets VX to the result of a bitwise and operation on a random number (Typically: 0 to 255) and NN.
	case 0xC000:
		x := opcode & 0x0F00 >> 8
		n := opcode & 0x00FF
		rand := uint8(12)
		registers[x] = uint8(n) & rand
		programCounter += 2
		return

	// DXYN - Draws a sprite at coordinate (VX, VY) that has a width of 8 pixels and a height of N pixels.
	// Each row of 8 pixels is read as bit-coded starting from memory location I;
	// I value doesn’t change after the execution of this instruction. As described above,
	// VF is set to 1 if any screen pixels are flipped from set to unset when the sprite is drawn, and to 0 if that doesn’t happen
	case 0xD000:
		x := opcode & 0x0F00 >> 8
		y := opcode & 0x00F0 >> 4
		rows := opcode & 0x000F

		xCoord := uint16(registers[x])
		yCoord := uint16(registers[y])

		registers[15] = 0

		for row := uint16(0); row < rows; row++ {

			spriteRow := memory[indexRegister+row]
			for i := 0; i < 8; i++ {
				bit := spriteRow & (0x80 >> i)
				pos := uint16(xCoord + uint16(i) + 64*(yCoord+row))

				if bit != 0 {
					if display[pos] {
						registers[15] = 1
					}
					display[pos] = !display[pos] && true
				}
			}
		}

		drawFlag = true
		programCounter += 2

		return

	case 0xE000:
		switch opcode & 0xF0FF {
		// EX9E - Skips the next instruction if the key stored in VX is pressed. (Usually the next instruction is a jump to skip a code block)
		case 0xE09E:
			x := opcode & 0x0F00 >> 8
			key := registers[x]
			if keys[rune(key)] {
				programCounter += 4
			} else {
				programCounter += 2
			}

			return
		// EXA1 - Skips the next instruction if the key stored in VX isn't pressed. (Usually the next instruction is a jump to skip a code block)
		case 0xE0A1:
			x := opcode & 0x0F00 >> 8
			key := registers[x]
			if !keys[rune(key)] {
				programCounter += 4
			} else {
				programCounter += 2
			}
			return
		}
	case 0xF000:
		switch opcode & 0xF0FF {

		// FX07 - Sets VX to the value of the delay timer.
		case 0xF007:
			x := opcode & 0x0F00 >> 8
			registers[x] = delayTimer
			programCounter += 2
			return

		// FX0A - A key press is awaited, and then stored in VX. (Blocking Operation. All instruction halted until next key event)
		case 0xF00A:
			programCounter += 2
			return

		// FX15 - Sets the delay timer to VX.
		case 0xF015:
			x := opcode & 0x0F00 >> 8
			delayTimer = registers[x]
			programCounter += 2
			return

		// FX18 - Sets the sound timer to VX.
		case 0xF018: // FX18
			x := opcode & 0x0F00 >> 8
			soundTimer = registers[x]
			programCounter += 2
			return

		// FX1E - Adds VX to I. VF is set to 1 when there is a range overflow (I+VX>0xFFF), and to 0 when there isn't.
		case 0xF01E: // FX1E

			x := opcode & 0x0F00 >> 8

			sum := uint16(indexRegister + uint16(registers[x]))

			if sum > 255 {
				registers[0xF] = 1
			} else {
				registers[0xF] = 0
			}

			indexRegister += uint16(registers[x])

			programCounter += 2
			return

		// FX29 - Sets I to the location of the sprite for the character in VX.
		// Characters 0-F (in hexadecimal) are represented by a 4x5 font.
		case 0xF029: // FX29
			x := uint8(opcode & 0x0F00 >> 8)
			indexRegister = uint16(registers[x] * 5)
			programCounter += 2
			return
		// FX33 - Stores the binary-coded decimal representation of VX, with the most significant of three digits at the address in I,
		// the middle digit at I plus 1, and the least significant digit at I plus 2. (In other words, take the decimal representation of VX,
		// place the hundreds digit in memory at location in I, the tens digit at location I+1, and the ones digit at location I+2.)
		case 0xF033:
			x := opcode & 0x0F00 >> 8
			d := registers[x]
			a := uint8(d / 100)
			b := uint8((d - a*100) / 10)
			c := uint8(d - a*100 - b*10)

			memory[indexRegister] = a
			memory[indexRegister+1] = b
			memory[indexRegister+2] = c

			programCounter += 2
			return

		// FX55 - Stores V0 to VX (including VX) in memory starting at address I.
		// The offset from I is increased by 1 for each value written, but I itself is left unmodified
		case 0xF055:
			x := uint16(opcode & 0x0F00 >> 8)
			for i := uint16(0); i <= x; i++ {
				memory[indexRegister+i] = registers[i]
			}
			programCounter += 2
			return

		// FX65 - Fills V0 to VX (including VX) with values from memory starting at address I.
		// The offset from I is increased by 1 for each value written, but I itself is left unmodified.
		case 0xF065:
			x := uint16(opcode & 0x0F00 >> 8)
			for i := uint16(0); i <= x; i++ {
				registers[i] = memory[indexRegister+i]
			}
			programCounter += 2
			return
		}
	}

	panic(fmt.Sprintf("Unsupported opcode: %X", opcode))
}

func emulate() {
	processOpcode()
	updateTimers()
}

func updateTimers() {
	if delayTimer > 0 {
		delayTimer--
	}

	if soundTimer > 0 {
		if soundTimer == 1 {
		}
		soundTimer--
	}
}

func render() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			if display[x+y*64] {
				termbox.SetCell(x, y, '*', termbox.ColorWhite, termbox.ColorDefault)
			} else {
				termbox.SetCell(x, y, ' ', termbox.ColorDefault, termbox.ColorDefault)
			}
		}
	}

	termbox.Flush()
}

func handleInput() {

}

func setupGraphics() {
	drawFlag = false
	fmt.Print("\033[H\033[2J")
}

func setupInput() {

}

func initialize() {

	programCounter = 0x200 //512
	opcode = 0
	indexRegister = 0
	stackPointer = 0

	// Clear display
	for i := 0; i < cap(display); i++ {
		display[i] = false
	}

	// Clear stack
	for i := 0; i < cap(stack); i++ {
		stack[i] = 0
	}

	// Clear registers V0-VF
	for i := 0; i < cap(registers); i++ {
		registers[i] = 0
	}

	// Clear memory
	for i := 0; i < cap(memory); i++ {
		memory[i] = 0
	}

	// Load fontset
	for i := 0; i < cap(fontset); i++ {
		memory[i] = fontset[i]
	}

}

func load(s string) {

	file, err := os.Open(s)
	if err != nil {
		log.Fatal(err)
	}

	memSlice := memory[512:]
	_, err = file.Read(memSlice)
	if err != nil {
		log.Fatal(err)
	}

	file.Close()
}
