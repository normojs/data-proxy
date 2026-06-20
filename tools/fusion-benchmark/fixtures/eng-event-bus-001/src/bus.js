export class EventBus {
  constructor() {
    this.handlers = new Map();
  }

  on(event, handler) {
    if (!this.handlers.has(event)) this.handlers.set(event, []);
    this.handlers.get(event).push(handler);
    return () => this.off(event, handler);
  }

  once(event, handler) {
    this.on(event, handler);
  }

  off(event, handler) {
    const list = this.handlers.get(event);
    if (!list) return;
    const index = list.indexOf(handler);
    if (index >= 0) list.splice(index, 1);
  }

  emit(event, payload) {
    const list = this.handlers.get(event) || [];
    for (const handler of list) {
      handler(payload);
    }
  }
}
