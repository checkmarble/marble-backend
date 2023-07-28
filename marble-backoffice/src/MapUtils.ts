export function MapObjectValues<ValueIn, ValueOut>(
  obj: { [key: string]: ValueIn },
  fn: (value: ValueIn) => ValueOut | undefined
): { [key: string]: ValueOut } {
  const p = Object.entries(obj)
    .map(([key, value]) => [key, fn(value)])
    .filter(([, value]) => value !== undefined);

  return Object.fromEntries(p);
}

export function MapMap<ValueIn, ValueOut>(
  mapIn: ReadonlyMap<string, ValueIn>,
  fn: (value: ValueIn) => ValueOut | undefined
): Map<string, ValueOut> {
  const result = new Map<string, ValueOut>();

  for (const [key, value] of mapIn.entries()) {
    const transformed = fn(value);
    if (transformed !== undefined) {
      result.set(key, transformed);
    }
  }
  return result;
}

export function ObjectToMap<Value>(obj: {
  [key: string]: Value;
}): Map<string, Value> {
  return new Map(Object.entries(obj));
}

export function MapToObject<Value>(map: Map<string, Value>): {
  [key: string]: Value;
} {
  return Object.fromEntries(map.entries());
}