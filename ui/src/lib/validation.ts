export function isValidID(id: string): boolean {
  if (!id) return false;
  return !/[\/\\]/.test(id);
}
