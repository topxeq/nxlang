package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/topxeq/nxlang/bytecode"
	"github.com/topxeq/nxlang/compiler"
	"github.com/topxeq/nxlang/parser"
	"github.com/topxeq/nxlang/stdlib"
	"github.com/topxeq/nxlang/vm"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		// Start REPL mode
		startREPL()
		return
	}

	switch args[0] {
	case "run":
		if len(args) < 2 {
			fmt.Println("Usage: nx run <file.nx|file.nxb> [args...]")
			os.Exit(1)
		}
		runFile(args[1], args[2:])
	case "compile":
		if len(args) < 2 {
			fmt.Println("Usage: nx compile <file.nx> [output.nxb]")
			os.Exit(1)
		}
		outputPath := ""
		if len(args) > 2 {
			outputPath = args[2]
		} else {
			outputPath = strings.TrimSuffix(args[1], filepath.Ext(args[1])) + ".nxb"
		}
		compileFile(args[1], outputPath)
	case "repl":
		startREPL()
	case "edit":
		fmt.Println("Built-in editor coming soon!")
	default:
		// Assume it's a file to run
		runFile(args[0], args[1:])
	}
}

// runFile executes a Nxlang file (either .nx source or .nxb bytecode)
func runFile(path string, scriptArgs []string) {
	ext := filepath.Ext(path)

	switch ext {
	case ".nx":
		// Run source file directly
		source, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			os.Exit(1)
		}

		// Parse
		lexer := parser.NewLexer(string(source))
		parser := parser.NewParser(lexer)
		program := parser.ParseProgram()

		if len(parser.Errors()) > 0 {
			fmt.Println("Parsing errors:")
			for _, err := range parser.Errors() {
				fmt.Printf("  %s\n", err)
			}
			os.Exit(1)
		}

		// Compile
		comp := compiler.NewCompiler()
		comp.ModulePath = path // Set the root module path
		if err := comp.Compile(program); err != nil {
			fmt.Printf("Compilation error: %v\n", err)
			os.Exit(1)
		}

		// Execute
		bytecode := comp.Bytecode()
		vm := vm.NewVM(bytecode)
		stdlib.RegisterAll(vm)
		vm.SetArgs(scriptArgs)

		if err := vm.Run(); err != nil {
			fmt.Printf("Runtime error: %v\n", err)
			os.Exit(1)
		}

	case ".nxb":
		// Run precompiled bytecode file
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading bytecode file: %v\n", err)
			os.Exit(1)
		}

		// Read bytecode
		reader := bytecode.NewReaderFromBytes(data)
		bc, err := reader.Read()
		if err != nil {
			fmt.Printf("Error parsing bytecode: %v\n", err)
			os.Exit(1)
		}

		// Execute
		vm := vm.NewVM(bc)
		stdlib.RegisterAll(vm)
		vm.SetArgs(scriptArgs)

		if err := vm.Run(); err != nil {
			fmt.Printf("Runtime error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("Unsupported file type: %s\n", ext)
		os.Exit(1)
	}
}

// compileFile compiles a .nx source file to .nxb bytecode
func compileFile(inputPath, outputPath string) {
	source, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Parse
	lexer := parser.NewLexer(string(source))
	parser := parser.NewParser(lexer)
	program := parser.ParseProgram()

	if len(parser.Errors()) > 0 {
		fmt.Println("Parsing errors:")
		for _, err := range parser.Errors() {
			fmt.Printf("  %s\n", err)
		}
		os.Exit(1)
	}

	// Compile
	comp := compiler.NewCompiler()
	comp.ModulePath = inputPath // Set the root module path
	if err := comp.Compile(program); err != nil {
		fmt.Printf("Compilation error: %v\n", err)
		os.Exit(1)
	}

	// Write bytecode to file
	bc := comp.Bytecode()
	writer := bytecode.NewWriter()
	if err := writer.Write(bc); err != nil {
		fmt.Printf("Error writing bytecode: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, writer.Bytes(), 0644); err != nil {
		fmt.Printf("Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully compiled to %s\n", outputPath)
}

// startREPL starts the interactive read-eval-print loop
func startREPL() {
	fmt.Println("Nxlang REPL (type 'exit' to quit)")
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		if input == "exit" || input == "quit" {
			break
		}

		// Parse
		lexer := parser.NewLexer(input)
		parser := parser.NewParser(lexer)
		program := parser.ParseProgram()

		if len(parser.Errors()) > 0 {
			for _, err := range parser.Errors() {
				fmt.Printf("Error: %s\n", err)
			}
			continue
		}

		// Compile
		comp := compiler.NewCompiler()
		if err := comp.Compile(program); err != nil {
			fmt.Printf("Compilation error: %v\n", err)
			continue
		}

		// Execute
		bytecode := comp.Bytecode()
		vm := vm.NewVM(bytecode)
		stdlib.RegisterAll(vm)

		if err := vm.Run(); err != nil {
			fmt.Printf("Runtime error: %v\n", err)
		}
	}

	fmt.Println("Goodbye!")
}
