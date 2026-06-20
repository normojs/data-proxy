export function parseCsv(text) {
  if (!text) return [];
  return text.split("\n").map((line) => line.split(","));
}
