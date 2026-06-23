export async function cookies() {
  return {
    get: (_name: string): { name: string; value: string } | undefined =>
      undefined,
    has: (_name: string): boolean => false,
    getAll: (): Array<{ name: string; value: string }> => [],
  };
}

export async function headers() {
  return new Headers();
}
