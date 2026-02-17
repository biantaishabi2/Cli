import { run } from "./pipeline";

export function walkDepth(level: number): void {
  if (level <= 0) {
    run();
    return;
  }

  walkDepth(level - 1);
}

export function startWalk(): void {
  walkDepth(32);
}
