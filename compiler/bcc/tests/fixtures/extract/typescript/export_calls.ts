export function run() {
  foo();
}

export class A {
  m() {
    this.bar();
    baz();
  }
}

export const nested = () => level1(level2(level3()));
