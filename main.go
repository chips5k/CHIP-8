package main

import (
	"fmt"
	"log"
	"os"
)

var (
	opcode         uint16
	memory         [4096]uint8
	registers      [16]uint8
	indexRegister  uint16
	programCounter uint16
	display        [64 * 32]bool
	delayTimer     int
	soundTimer     int
	stack          []uint16
	stackPointer   uint16
	key            [16]byte
	ticks          int
	drawFlag       bool
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

func main() {

	setupGraphics()
	setupInput()

	initialize()
	load("pong")

	running := true

	for running == true {

		emulate()

		// If draw flag set
		if drawFlag {
			render()
			drawFlag = false
		}

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
			programCounter = stack[stackPointer]
			stackPointer--
			return
		// Calls RCA 1802 program at address NNN. Not necessary for most ROMs. skipping impl. but could use 2NNN i think....
		default:
		}

	// 1NNN - Jumps to address NNN.
	case 0x1000:
		programCounter = 0x0FFF
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
			programCounter += 4 // this could be wrong, might be a simple case of +2 here, and no else...
		} else {
			programCounter += 2
		}

		return
	// 4XNN	Skips the next instruction if VX doesn't equal NN. (Usually the next instruction is a jump to skip a code block)
	case 0x4000: // 4XNN
		x := opcode & 0x0F00 >> 8
		n := uint8(opcode & 0x00FF)

		if registers[x] != n {
			programCounter += 4 // this could be wrong, might be a simple case of +2 here, and no else...
		} else {
			programCounter += 2
		}
		return

	// 5XY0	Skips the next instruction if VX equals VY. (Usually the next instruction is a jump to skip a code block)
	case 0x5000: // 5XY0
		x := opcode & 0x0F00 >> 8
		y := opcode & 0x00F0 >> 4

		if registers[x] == registers[y] {
			programCounter += 4 // this could be wrong, might be a simple case of +2 here, and no else...
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
		case 0x8005:

		// 8XY6 - Stores the least significant bit of VX in VF and then shifts VX to the right by 1.
		case 0x8006:

		// 8XY7 - Sets VX to VY minus VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
		case 0x8007:

		// 8XYE - Stores the most significant bit of VX in VF and then shifts VX to the left by 1.
		case 0x800E:

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
		drawFlag = true
		x := opcode & 0x0F00 >> 8
		y := opcode & 0x00F0 >> 4
		rows := opcode & 0x000F

		xCoord := registers[x]
		yCoord := registers[y]

		registers[15] = 0

		for row := uint16(0); row < rows; row++ {

			spriteRow := memory[indexRegister+row]

			for i := 0; i < 8; i++ {
				bit := spriteRow & (0x80 >> i)

				if bit != 0 {
					if display[xCoord+yCoord*64] {
						registers[15] = 1
					}
					display[xCoord+yCoord*64] = true
				} else {
					display[xCoord+yCoord*64] = false
				}
			}
		}

		//programCounter += 2
		return

	case 0xE000:
		switch opcode & 0xF0FF {
		// EX9E - Skips the next instruction if the key stored in VX is pressed. (Usually the next instruction is a jump to skip a code block)
		case 0xE09E:
			return

			// EXA1 - Skips the next instruction if the key stored in VX isn't pressed. (Usually the next instruction is a jump to skip a code block)
		case 0xE0A1:
			return
		}
	case 0xF000:
		switch opcode & 0xF0FF {

		// FX07 - Sets VX to the value of the delay timer.
		case 0xF007:
			return
		// FX0A - A key press is awaited, and then stored in VX. (Blocking Operation. All instruction halted until next key event)
		case 0xF00A:
			return

		// FX15 - Sets the delay timer to VX.
		case 0xF015:
			return

		// FX18 - Sets the sound timer to VX.
		case 0xF018: // FX18
			return

		// FX1E - Adds VX to I. VF is set to 1 when there is a range overflow (I+VX>0xFFF), and to 0 when there isn't.
		case 0xF01E: // FX1E
			return

		// FX29 - Sets I to the location of the sprite for the character in VX. Characters 0-F (in hexadecimal) are represented by a 4x5 font.
		case 0xF029: // FX29
			return

		// FX33 - Stores the binary-coded decimal representation of VX, with the most significant of three digits at the address in I,
		// the middle digit at I plus 1, and the least significant digit at I plus 2. (In other words, take the decimal representation of VX,
		// place the hundreds digit in memory at location in I, the tens digit at location I+1, and the ones digit at location I+2.)
		case 0xF033:
			return

		// FX55
		case 0xF055:
			return

		// FX65
		case 0xF065:
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
			fmt.Println("BEEP!")
		}
		soundTimer--
	}
}

func render() {
	fmt.Println("\033[33A")
	buf := ""
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			if display[x+y*64] {
				buf = fmt.Sprintf("%s*", buf)
			} else {
				buf = fmt.Sprintf("%s ", buf)
			}
		}
		buf = fmt.Sprintf("%s\n", buf)
	}

	fmt.Printf("%s", buf)

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

	file, err := os.Open("roms/PONG")
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
