public class Fib {
    public static long fib(int n) {
        if (n <= 1) {
            return n;
        }
        return fib(n - 1) + fib(n - 2);
    }

    public static void main(String[] args) {
        long start = System.currentTimeMillis();
        long result = fib(30);
        long end = System.currentTimeMillis();
        System.out.println("Fibonacci(30) =" + result);
        System.out.println("Time (ms):" + (end - start));
    }
}
