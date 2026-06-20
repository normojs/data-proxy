export function buildUrl(base, path, query = {}) {
  const url = `${base}/${path}`;
  const params = new URLSearchParams(query);
  const suffix = params.toString();
  return suffix ? `${url}?${suffix}` : url;
}
