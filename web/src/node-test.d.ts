// Minimal typings for the node:test and node:assert APIs the tests use.
declare module "node:test" {
  type Fn = () => void | Promise<void>;
  export function describe(name: string, fn: () => void): void;
  export function it(name: string, fn: Fn): void;
  export function beforeEach(fn: Fn): void;
  export function afterEach(fn: Fn): void;
}

declare module "node:assert/strict" {
  const assert: {
    ok(value: unknown, message?: string): void;
    equal(actual: unknown, expected: unknown, message?: string): void;
    notEqual(actual: unknown, expected: unknown, message?: string): void;
    deepEqual(actual: unknown, expected: unknown, message?: string): void;
    match(actual: string, expected: RegExp, message?: string): void;
    throws(fn: () => unknown, expected?: RegExp): void;
  };
  export default assert;
}
