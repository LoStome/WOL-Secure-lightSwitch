export const getErrorMessage = (error: unknown): string | undefined =>
  error instanceof Error ? error.message : undefined;
