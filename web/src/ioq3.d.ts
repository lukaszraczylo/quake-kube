// Globals provided by ioquake3.js (QuakeJS fork).
interface Ioq3 {
  canvas: HTMLCanvasElement | null;
  viewport: HTMLElement | null;
  elementPointerLock: boolean;
  exitHandler: ((err?: unknown) => void) | null;
  setCanvasSize(width: number, height: number): void;
  callMain(args: string[]): void;
}

interface EmscriptenModule {
  TOTAL_MEMORY: number;
  canvas: HTMLCanvasElement | null;
  printErr(text: string): void;
  setStatus(text: string): void;
}

declare const ioq3: Ioq3;

interface Window {
  Module: EmscriptenModule;
}
