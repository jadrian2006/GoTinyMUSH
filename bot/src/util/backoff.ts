export class Backoff {
  private attempt = 0;

  constructor(
    private readonly baseMs: number = 1000,
    private readonly maxMs: number = 30000,
    private readonly jitterFactor: number = 0.3
  ) {}

  next(): number {
    const exp = Math.min(this.baseMs * 2 ** this.attempt, this.maxMs);
    const jitter = exp * this.jitterFactor * (Math.random() * 2 - 1);
    this.attempt++;
    return Math.max(0, Math.round(exp + jitter));
  }

  reset(): void {
    this.attempt = 0;
  }
}
