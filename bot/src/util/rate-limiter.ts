export class RateLimiter {
  private lastResponse = new Map<string, number>();

  constructor(private readonly cooldownMs: number) {}

  check(contextKey: string): boolean {
    const last = this.lastResponse.get(contextKey);
    if (last && Date.now() - last < this.cooldownMs) {
      return false;
    }
    return true;
  }

  mark(contextKey: string): void {
    this.lastResponse.set(contextKey, Date.now());
  }
}
