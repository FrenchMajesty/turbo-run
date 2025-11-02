import { NodeData } from '@/types/NodeData';
import { eventBus } from './eventBus';

type Comparator<T> = (a: ItemWithPriority<T>, b: ItemWithPriority<T>) => boolean;
type ItemWithPriority<T> = {
    item: T;
    priority: number;
}

export class Heap<T> {
    public data: ItemWithPriority<T>[];
    private comparator: Comparator<T>;

    constructor(comparator: Comparator<T>) {
        this.data = [];
        this.comparator = comparator;
    }

    private parent(index: number): number {
        return Math.floor((index - 1) / 2);
    }

    private leftChild(index: number): number {
        return 2 * index + 1;
    }

    private rightChild(index: number): number {
        return 2 * index + 2;
    }

    private stripPriority(item: ItemWithPriority<T>): T {
        return item.item;
    }

    private swap(index1: number, index2: number): void {
        [this.data[index1], this.data[index2]] = [this.data[index2], this.data[index1]];
        eventBus.publish('heap.swap', { index1, index2, data: this.data.map(this.stripPriority) });
    }

    private heapifyUp(index: number): void {
        while (index > 0 && this.comparator(this.data[index], this.data[this.parent(index)])) {
            this.swap(index, this.parent(index));
            index = this.parent(index);
        }
    }

    private heapifyDown(index: number): void {
        let left = this.leftChild(index);
        let right = this.rightChild(index);
        let smallest = index;

        if (left < this.data.length && this.comparator(this.data[left], this.data[smallest])) {
            smallest = left;
        }

        if (right < this.data.length && this.comparator(this.data[right], this.data[smallest])) {
            smallest = right;
        }

        if (smallest !== index) {
            this.swap(index, smallest);
            this.heapifyDown(smallest);
        }
    }

    /**
     * Pushes an item into the heap.
     * @param item - The item to insert into the heap.
     */
    push(item: T, priority: number): void {
        this.data.push({ item, priority });
        this.heapifyUp(this.data.length - 1);
        eventBus.publish('heap.push', this.data.map(this.stripPriority));
    }

    /**
     * Pop the root item from the heap.
     * @returns The root item or null if the heap is empty.
     */
    pop(): T | null {
        if (this.data.length === 0) return null;
        const root = this.data[0];
        const last = this.data.pop()!;
        if (this.data.length > 0) {
            this.data[0] = last;
            this.heapifyDown(0);
        }
        eventBus.publish('heap.pop', this.data.map(this.stripPriority));
        return root.item;
    }

    /**
     * Peek the root item from the heap.
     * @returns The root item or null if the heap is empty.
     */
    peek(): T | null {
        return this.data.length > 0 ? this.data[0].item : null;
    }

    /**
     * Get the size of the heap.
     * @returns The size of the heap.
     */
    size(): number {
        return this.data.length;
    }

    /**
     * Check if the heap is empty.
     * @returns True if the heap is empty, false otherwise.
     */
    isEmpty(): boolean {
        return this.data.length === 0;
    }
}


export class MaxHeap<T> extends Heap<T> {
    constructor() {
        super((a, b) => a.priority > b.priority);
    }
}

export class MinHeap<T> extends Heap<T> {
    constructor() {
        super((a, b) => a.priority < b.priority);
    }
}

export const priorityQueue = new MaxHeap<NodeData>();
