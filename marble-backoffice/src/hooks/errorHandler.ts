
export function presentAsyncError(f: () => Promise<void>) : void {
  (async () => {
    try {
      await f();
    } catch (error: unknown) {
      // TODO: store error
      throw error;
    }
  })();
}
