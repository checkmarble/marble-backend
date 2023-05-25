import { useReducer, Dispatch } from "react";

enum CountActionKind {
  INCREMENT,
  DECREMENT,
}

interface CountAction {
  type: CountActionKind;
}

function counterReducer(counter: number, action: CountAction) {
  const { type } = action;
  switch (type) {
    case CountActionKind.INCREMENT:
      return counter + 1;
    case CountActionKind.DECREMENT:
      return counter - 1;
  }
}

export type LoadingDispatcher = Dispatch<CountAction>;

export function useLoading(): [boolean, LoadingDispatcher] {
  const [loadingCounter, dispatch] = useReducer(counterReducer, 0);

  const loading = loadingCounter != 0;

  return [loading, dispatch];
}

export async function showLoader<T>(
  dispatch: LoadingDispatcher,
  promise: Promise<T>
): Promise<T> {
  dispatch({ type: CountActionKind.INCREMENT });
  try {
    // await new Promise((resolve) => setTimeout(resolve, 3000));
    return await promise;
  } finally {
    dispatch({ type: CountActionKind.DECREMENT });
  }
}
