import { useCallback, useEffect, useState } from "react";
import { type LoadingDispatcher, showLoader } from "@/hooks/Loading";
import { presentAsyncError } from "./errorHandler";

export function useRunOnce(loader: () => Promise<void>) {
  useEffect(() => {
    presentAsyncError(loader);
  }, [loader]);
}

export function useSimpleLoader<Things>(
  loadingDispatcher: LoadingDispatcher,
  loader: () => Promise<Things | null>
): [Things | null, () => Promise<void>] {
  const [things, setThings] = useState<Things | null>(null);

  const fetch = useCallback(async () => {
    setThings(await showLoader(loadingDispatcher, loader()));
  }, [loadingDispatcher, loader]);

  useRunOnce(fetch);

  return [things, fetch];
}
