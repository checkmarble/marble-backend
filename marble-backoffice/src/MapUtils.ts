export function MapObjectValues<ValueIn, ValueOut>(
  obj: { [key: string]: ValueIn },
  fn: (value: ValueIn) => ValueOut
): { [key: string]: ValueOut } {
  return Object.fromEntries(
    Object.entries(obj).map(([key, value]) => [key, fn(value)])
  );
}
