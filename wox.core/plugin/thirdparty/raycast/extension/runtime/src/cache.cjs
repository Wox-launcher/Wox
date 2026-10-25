const fs = require('node:fs');
const path = require('node:path');

// Cache instances in one command share namespace state, including subscriptions used by @raycast/utils.
// Persistence stays in Wox's plugin cache directory; React updates use the normal render bridge.
function createCache(supportPath) {
  const namespaces = new Map();
  return class Cache {
    constructor(options = {}) {
      const namespace = options.namespace || '';
      if (!namespaces.has(namespace)) {
        const file = path.join(supportPath, `cache-${Buffer.from(namespace).toString('hex')}.json`);
        const data = Object.create(null);
        try { Object.assign(data, JSON.parse(fs.readFileSync(file, 'utf8'))); } catch {}
        namespaces.set(namespace, { file, data, subscribers: new Set() });
      }
      this.state = namespaces.get(namespace);
    }
    get(key) { return this.state.data[key]; }
    has(key) { return Object.hasOwn(this.state.data, key); }
    get isEmpty() { return Object.keys(this.state.data).length === 0; }

    // Subscribe is passed directly to React.useSyncExternalStore, without a bound receiver.
    subscribe = subscriber => {
      const subscription = { subscriber };
      this.state.subscribers.add(subscription);
      return () => this.state.subscribers.delete(subscription);
    };

    set(key, value) {
      this.save({ ...this.state.data, [key]: value });
      this.notify(key, value);
    }
    remove(key) {
      if (!this.has(key)) return false;
      const data = { ...this.state.data };
      delete data[key];
      this.save(data);
      this.notify(key, undefined);
      return true;
    }
    clear(options = {}) {
      this.save({});
      if (options.notifySubscribers !== false) this.notify(undefined, undefined);
    }

    // Publish the new snapshot only after the disk write succeeds; readers never see a partial file.
    save(data) {
      fs.mkdirSync(supportPath, { recursive: true });
      const temporary = `${this.state.file}.${process.pid}.tmp`;
      try {
        fs.writeFileSync(temporary, JSON.stringify(data));
        fs.renameSync(temporary, this.state.file);
      } finally {
        fs.rmSync(temporary, { force: true });
      }
      this.state.data = Object.assign(Object.create(null), data);
    }

    // Snapshot listeners so callbacks can unsubscribe or add listeners without extending this delivery.
    notify(key, value) {
      for (const subscription of [...this.state.subscribers]) {
        if (this.state.subscribers.has(subscription)) subscription.subscriber(key, value);
      }
    }
  };
}

module.exports = { createCache };
