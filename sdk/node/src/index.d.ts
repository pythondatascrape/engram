export class EngramError extends Error {
  code?: number;
  constructor(message: string, code?: number);
}

export class Engram {
  static connect(socketPath?: string): Promise<Engram>;
  compress(context: Record<string, unknown>): Promise<Record<string, unknown>>;
  deriveCodebook(content: string): Promise<Record<string, unknown>>;
  getStats(): Promise<Record<string, unknown>>;
  checkRedundancy(content: string): Promise<Record<string, unknown>>;
  generateReport(): Promise<Record<string, unknown>>;
  close(): Promise<void>;
}
