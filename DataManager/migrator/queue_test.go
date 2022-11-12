package migrate

import "testing"

func checkQueueLength(t *testing.T, queue fileQueue, expected int) {
	if len(queue) != expected {
		t.Fatalf("expected a queue length of %d but got %d", expected, len(queue))
	}

}

func checkQueueValue(t *testing.T, expected, got fileIdentifier) {
	if expected != got {
		t.Fatalf("dequeued incorrect value %s expected %s", got, expected)
	}
}

func TestQueue(t *testing.T) {
	var queue fileQueue
	queue.enqueue("test")
	queue.enqueue("test2")
	queue.enqueue("test3")

	if !queue.next() {
		t.Fatal("expected queue next to be true")
	}

	checkQueueLength(t, queue, 3)

	checkQueueValue(t, "test", queue.dequeue())
	checkQueueLength(t, queue, 2)

	checkQueueValue(t, "test2", queue.dequeue())
	checkQueueLength(t, queue, 1)

	checkQueueValue(t, "test3", queue.dequeue())
	checkQueueLength(t, queue, 0)

	if queue.next() {
		t.Fatal("expected queue next to be false")
	}
}
