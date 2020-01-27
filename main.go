package main

import (
	"fmt"
	"log"
	"os"
)

var (
	opcode         uint16
	memory         [4096]byte
	registers      [16]uint16
	indexRegister  uint16
	programCounter uint16
	display        [64 * 32]bool
	delayTimer     int
	soundTimer     int
	stack          [16]uint16
	stackPointer   uint16
	key            [16]byte
	ticks          int
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

	fmt.Println("CHIP-8 emulator")

	setupGraphics()
	setupInput()

	initialize()
	load("pong")

	running := true

	for running == true && ticks < 255 {
		fmt.Println("Emulating....")

		emulate()

		// If draw flag set
		if registers[15] != 0 {
			render()
		}

		handleInput()
		ticks++
	}

	fmt.Println("Exiting...")

}

func processOpcode() {
	//Fetch opcode
	// memory is 4096 bytes, opcode is 2 bytes, so we pull two addresses and combine them
	// Shift first to the left 8bits, or it with the next
	opcode = uint16(memory[programCounter])<<8 | uint16(memory[programCounter+1])

	digits := [...]uint16{
		opcode & 0xF000 >> 12,
		opcode & 0x0F00 >> 8,
		opcode & 0x00F0 >> 4,
		opcode & 0x000F,
	}

	fmt.Printf("%X\n", digits)

	masked := [...]uint16{
		opcode & 0xF000,
		opcode & 0xF00F,
		opcode & 0xF0FF,
	}

	switch masked[0] {

	case 0x0000:
		switch opcode {
		case 0x00E0: // 00E0

		case 0x00EE: // 00EE
		default: // 0NNN

		}
	case 0x1000: // 1NNN

	case 0x2000: // 2NNN
		//Run a subroutine,
		//First store the current prog counter in the stack so we can track it later
		stack[stackPointer] = programCounter
		//Bump the stack pointer (same thing as we do with prog counter)
		stackPointer++
		//set program counter to point to the subroutine
		programCounter = opcode & 0x0FFF
		//^ Assume when the subroutine flow finishes, we pop the stack onto the prog counter and continue
		return
	case 0x3000: // 3XNN

	case 0x4000: // 4XNN

	case 0x5000: // 5XY0

	case 0x6000: // 6XNN
		// Sets VX to NN
		//Execute Opcode
		registers[digits[1]] = digits[2] + digits[3]
		//Bump the program counter
		programCounter += 2
		return

	case 0x7000: // 7XNN

	case 0x8000:
		switch masked[1] {
		case 0x8000: // 8XY0

		case 0x8001: // 8XY1

		case 0x8002: // 8XY2

		case 0x8003: // 8XY3

		case 0x8004: // 8XY4

		case 0x8005: // 8XY5

		case 0x8006: // 8XY6

		case 0x8007: // 8XY7

		case 0x800E: // 8XYE

		}
	case 0x9000: // 9XY0

	case 0xA000: // ANNN
		// ANNN Sets I to the address NNN
		//Execute Opcode
		indexRegister = opcode & 0x0FFF //zero out A?
		//Bump the program counter
		programCounter += 2
		return
	case 0xC000: // CXNN

	case 0xD000: // DXYN

	case 0xE000:
		switch masked[2] {
		case 0xE09E: // EX9E

		case 0xE0A1: // EXA1

		}
	case 0xF000:
		switch masked[2] {
		case 0xF007: // FX07

		case 0xF00A: // FX0A

		case 0xF015: // FX15

		case 0xF018: // FX18

		case 0xF01E: // FX1E

		case 0xF029: // FX29

		case 0xF033: // FX33

		case 0xF055: // FX55

		case 0xF065: // FX65

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

}

func handleInput() {

}

func setupGraphics() {

}

func setupInput() {

}

func initialize() {

	fmt.Println("Initializing")

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

	fmt.Println("Loading rom: roms/PONG...")

	file, err := os.Open("roms/PONG")
	if err != nil {
		log.Fatal(err)
	}

	memSlice := memory[512:]
	count, err := file.Read(memSlice)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Read %d bytes: %q\n", count, memSlice[:count])
	file.Close()

}
