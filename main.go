package main

import (
	"fmt"
	"os"
	"log"
)

/*
64/32 (display res)

4096 memory

addresses start at 512 typically (but modern can use it, most use it for fontset data)

top 256 bytes == display refresh

96 bytes below === call stack, internal, other vars

*/

var (
	opcode         uint16
	memory         [4096]byte
	registers      [16]byte
	indexRegister  uint16
	programCounter uint16
	display        [64 * 32]bool
	delayTimer     int
	soundTimer     int
	stack          [16]uint16
	stackPointer   uint16
	key            [16]byte
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

	for running == true {
		fmt.Println("Emulating....")

		emulate()

		// If draw flag set
		if registers[15] != 0 {
			render()
		}

		running = false

		handleInput()
	}

}

func emulate() {
	//Fetch opcode

	// memory is 4096 bytes, opcode is 2 bytes, so we pull two addresses and combine them
	// Shift first to the left 8bits, or it with the next
	opcode = uint16(memory[programCounter])<<8 | uint16(memory[programCounter+1])

	//Decode opcode

	//Execute Opcode

	//Bump the program counter
	programCounter += 2

	//Update timers
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

	programCounter = 0x200
	opcode = 0
	indexRegister = 0
	stackPointer = 0

	// Clear display

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