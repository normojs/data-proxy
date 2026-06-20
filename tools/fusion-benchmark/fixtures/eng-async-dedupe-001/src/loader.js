export class CoalescingLoader {
  constructor(fetcher) {
    this.fetcher = fetcher;
    this.cache = new Map();
  }

  async load(key) {
    if (this.cache.has(key)) {
      return this.cache.get(key);
    }
    const value = await this.fetcher(key);
    this.cache.set(key, value);
    return value;
  }

  clear(key) {
    this.cache.delete(key);
  }
}
