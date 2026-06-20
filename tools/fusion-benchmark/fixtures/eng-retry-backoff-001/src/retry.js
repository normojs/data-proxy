export async function retry(fn, options = {}) {
  const retries = options.retries ?? 2;
  const shouldRetry = options.shouldRetry ?? (() => true);
  const wait = options.wait ?? (() => Promise.resolve());
  let lastError;

  for (let attempt = 0; attempt < retries; attempt += 1) {
    try {
      return await fn(attempt);
    } catch (err) {
      lastError = err;
      if (!shouldRetry(err, attempt)) throw err;
      await wait(attempt);
    }
  }

  throw lastError;
}
