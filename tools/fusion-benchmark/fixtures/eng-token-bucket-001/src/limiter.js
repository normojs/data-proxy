export class TokenBucket {
  constructor({ capacity, refillPerMs, now = () => Date.now() }) {
    this.capacity = capacity;
    this.refillPerMs = refillPerMs;
    this.now = now;
    this.tokens = capacity;
    this.updatedAt = now();
  }

  allow(cost = 1) {
    const current = this.now();
    const elapsed = current - this.updatedAt;
    this.tokens += elapsed * this.refillPerMs;
    this.updatedAt = current;

    if (this.tokens < cost) return false;
    this.tokens -= cost;
    return true;
  }
}
