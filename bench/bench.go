package main

import (
	"fmt"
	"time"
)

func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	// Fibonacci
	start := time.Now()
	result := fib(30)
	elapsed := time.Since(start)
	fmt.Printf("Fibonacci(30) = %d\n", result)
	fmt.Printf("Time: %v\n", elapsed)

	// Loop
	iterations := 1000000
	start = time.Now()
	sum := 0
	for i := 0; i < iterations; i++ {
		sum += i
	}
	elapsed = time.Since(start)
	fmt.Printf("\nLoop sum: %d\n", sum)
	fmt.Printf("Iterations: %d\n", iterations)
	fmt.Printf("Time: %v	// Array operations\n", elapsed)

	size := 10000
	start = time.Now()

	arr := make([]int, size)
	for i := 0; i < size; i++ {
		arr[i] = i
	}

	arrSum := 0
	for _, v := range arr {
		arrSum += v
	}

	// Simple bubble sort
	sorted := make([]int, len(arr))
	copy(sorted, arr)
	n := len(sorted)
	for i := 0; i < n; i++ {
		for j := 0; j < n-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	elapsed = time.Since(start)
	fmt.Printf("\nArray size: %d\n", size)
	fmt.Printf("Sum: %d\n", arrSum)
	fmt.Printf("Sorted[0], Sorted[last]: %d, %d\n", sorted[0], sorted[len(sorted)-1])
	fmt.Printf("Time: %v\n", elapsed)

	// Function calls
	iterations = 500000
	start = time.Now()

	add := func(a, b int) int {
		return a + b
	}

	result = 0
	for i := 0; i < iterations; i++ {
		result = add(i, i+1)
	}
	elapsed = time.Since(start)
	fmt.Printf("\nFunction calls: %d\n", iterations)
	fmt.Printf("Result: %d\n", result)
	fmt.Printf("Time: %v\n", elapsed)
}
