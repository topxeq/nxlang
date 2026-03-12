import time

# Fibonacci
def fib(n):
    if n <= 1:
        return n
    return fib(n - 1) + fib(n - 2)

start = time.time()
result = fib(30)
elapsed = time.time() - start
print(f"Fibonacci(30) = {result}")
print(f"Time: {elapsed:.3f}s")

# Loop
iterations = 1000000
start = time.time()
sum_val = 0
for i in range(iterations):
    sum_val += i
elapsed = time.time() - start
print(f"\nLoop sum: {sum_val}")
print(f"Iterations: {iterations}")
print(f"Time: {elapsed:.3f}s")

# Array operations
size = 10000
start = time.time()

arr = list(range(size))

arr_sum = sum(arr)

# Sort
sorted_arr = sorted(arr)

elapsed = time.time() - start
print(f"\nArray size: {size}")
print(f"Sum: {arr_sum}")
print(f"Sorted[0], Sorted[last]: {sorted_arr[0]}, {sorted_arr[-1]}")
print(f"Time: {elapsed:.3f}s")

# Function calls
iterations = 500000

def add(a, b):
    return a + b

start = time.time()
result = 0
for i in range(iterations):
    result = add(i, i + 1)
elapsed = time.time() - start
print(f"\nFunction calls: {iterations}")
print(f"Result: {result}")
print(f"Time: {elapsed:.3f}s")
