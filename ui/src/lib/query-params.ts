export function validateSearchParams(
  searchParams: URLSearchParams,
  allowed: Set<string>,
): URLSearchParams {
  const cleaned = new URLSearchParams();
  for (const [key, value] of searchParams.entries()) {
    if (!allowed.has(key)) {
      continue;
    }
    cleaned.set(key, value);
  }
  return cleaned;
}
