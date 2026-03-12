public class Loop {
    public static void main(String[] args) {
        int iterations = 1000000;
        long start = System.currentTimeMillis();
        long sum = 0;
        for (int i = 0; i < iterations; i++) {
            sum = sum + i;
        }
        long end = System.currentTimeMillis();
        System.out.println("Loop sum:" + sum);
        System.out.println("Iterations:" + iterations);
        System.out.println("Time (ms):" + (end - start));
    }
}
