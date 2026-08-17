/**
 * Narrowing helper shared by the resolvers.
 *
 * Every resolver field is optional and every one must be checked for finiteness
 * before use, so this pairs the two into one type guard. Written as a predicate
 * rather than a bare `Number.isFinite` call because `isFinite` returns
 * `boolean` and leaves the value typed `number | undefined`, forcing a cast at
 * each use site — and a cast is exactly the thing that would let a genuinely
 * undefined value through if a check were ever moved or dropped.
 *
 * NaN is treated as absent. It arrives from a proto default or a division that
 * went wrong upstream, and neither is a reading worth rendering.
 */
export function isNum(value: number | undefined): value is number {
  return value !== undefined && Number.isFinite(value);
}
