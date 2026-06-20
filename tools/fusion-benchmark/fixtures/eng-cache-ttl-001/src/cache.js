export class TtlCache {
  constructor(now = () => Date.now()) {
    this.now = now;
    this.items = new Map();
  }

  set(key, value, ttlMs) {
    this.items.set(key, {
      value,
      expiresAt: this.now() + ttlMs
    });
  }

  get(key) {
    const item = this.items.get(key);
    if (!item) return undefined;
    return item.value;
  }

  has(key) {
    return this.get(key) !== undefined;
  }
}
