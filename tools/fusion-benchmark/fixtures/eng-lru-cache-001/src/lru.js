export class LruCache {
  constructor(capacity) {
    this.capacity = capacity;
    this.items = new Map();
  }

  get(key) {
    return this.items.get(key);
  }

  set(key, value) {
    this.items.set(key, value);
    if (this.items.size > this.capacity) {
      const oldest = this.items.keys().next().value;
      this.items.delete(oldest);
    }
  }

  has(key) {
    return this.items.has(key);
  }
}
