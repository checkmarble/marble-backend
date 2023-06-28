export function MapObjectValues<ValueIn, ValueOut>(
  obj: { [key: string]: ValueIn },
  fn: (value: ValueIn) => ValueOut | undefined
): { [key: string]: ValueOut } {

  const p = Object.entries(obj).
    map(([key, value]) => [key, fn(value)]).
    filter(([, value]) => value !== undefined);
  
  return Object.fromEntries(p);
}
