// Small DOM fake for the tests: only what ui.ts, player.ts and main.ts touch.
type Handler = (ev: FakeEvent) => void;

export class FakeEvent {
  defaultPrevented = false;
  readonly type: string;
  constructor(type: string) {
    this.type = type;
  }
  preventDefault(): void {
    this.defaultPrevented = true;
  }
}

class Observer {
  readonly cb: () => void;
  target: FakeElement | null = null;
  constructor(cb: () => void) {
    this.cb = cb;
  }
  observe(target: FakeElement): void {
    this.target = target;
    observers.add(this);
  }
  disconnect(): void {
    observers.delete(this);
  }
}

const observers = new Set<Observer>();

function notify(el: FakeElement): void {
  for (const o of observers) if (o.target && el.within(o.target)) queueMicrotask(o.cb);
}

function matches(el: FakeElement, selector: string): boolean {
  if (selector.startsWith("#")) return el.id === selector.slice(1);
  if (selector.startsWith(".")) return el.className.split(" ").includes(selector.slice(1));
  return el.tag === selector;
}

export class FakeElement {
  id = "";
  className = "";
  value = "";
  placeholder = "";
  href = "";
  offsetWidth = 800;
  offsetHeight = 600;
  children: FakeElement[] = [];
  parent: FakeElement | null = null;
  #text = "";
  readonly style: Record<string, string>;
  #handlers = new Map<string, Handler[]>();

  readonly tag: string;
  constructor(tag: string) {
    this.tag = tag;
    this.style = new Proxy<Record<string, string>>({}, {
      set: (t, k: string, v: string) => {
        t[k] = v;
        notify(this);
        return true;
      },
    });
  }

  set innerHTML(_: string) {
    throw new Error("innerHTML is not allowed: use textContent");
  }

  get textContent(): string {
    return this.#text + this.children.map((c) => c.textContent).join("");
  }
  set textContent(v: string) {
    this.#text = v;
    this.children = [];
    notify(this);
  }

  appendChild(child: FakeElement): FakeElement {
    child.parent = this;
    this.children.push(child);
    notify(this);
    return child;
  }
  replaceChildren(...kids: FakeElement[]): void {
    this.#text = "";
    this.children = [];
    kids.forEach((k) => this.appendChild(k));
  }
  remove(): void {
    if (!this.parent) return;
    this.parent.children = this.parent.children.filter((c) => c !== this);
    this.parent = null;
  }
  querySelector(selector: string): FakeElement | null {
    for (const c of this.children) {
      if (matches(c, selector)) return c;
      const hit = c.querySelector(selector);
      if (hit) return hit;
    }
    return null;
  }
  addEventListener(type: string, fn: Handler): void {
    this.#handlers.set(type, [...(this.#handlers.get(type) ?? []), fn]);
  }
  dispatch(type: string): FakeEvent {
    const ev = new FakeEvent(type);
    for (const fn of this.#handlers.get(type) ?? []) fn(ev);
    return ev;
  }
  requestSubmit(): void {
    this.dispatch("submit");
  }

  within(root: FakeElement): boolean {
    for (let n: FakeElement | null = this; n; n = n.parent) if (n === root) return true;
    return false;
  }
}

export class FakeStorage {
  readonly data = new Map<string, string>();
  failGet = false;
  failSet = false;
  getItem(k: string): string | null {
    if (this.failGet) throw new Error("storage denied");
    return this.data.get(k) ?? null;
  }
  setItem(k: string, v: string): void {
    if (this.failSet) throw new Error("storage quota");
    this.data.set(k, v);
  }
  clear(): void {
    this.data.clear();
  }
}

export interface Env {
  byId: Map<string, FakeElement>;
  storage: FakeStorage;
  win: FakeElement & { location: { host: string; protocol: string; search: string; href: string }; Module?: unknown };
  doc: Record<string, unknown>;
}

/** Installs fresh globals; call once per test. */
export function install(opts: { password?: boolean; search?: string } = {}): Env {
  observers.clear();
  const byId = new Map<string, FakeElement>();
  const make = (tag: string, id: string, parent?: FakeElement): FakeElement => {
    const el = new FakeElement(tag);
    el.id = id;
    if (id) byId.set(id, el);
    parent?.appendChild(el);
    return el;
  };
  make("div", "status");
  const main = make("div", "main");
  const form = make("form", "join-form", main);
  make("input", "playerName", form);
  if (opts.password) make("input", "password", form);
  make("div", "viewport-frame");

  const storage = new FakeStorage();
  const win = Object.assign(new FakeElement("window"), {
    location: { host: "game.test", protocol: "http:", search: opts.search ?? "", href: "" },
  });
  const doc: Record<string, unknown> = {
    getElementById: (id: string) => byId.get(id) ?? null,
    createElement: (tag: string) => new FakeElement(tag),
  };

  const g = globalThis as Record<string, unknown>;
  g.document = doc;
  g.window = win;
  g.localStorage = storage;
  g.MutationObserver = Observer;
  Object.defineProperty(doc, "location", { value: win.location });
  return { byId, storage, win, doc };
}

export const tick = (): Promise<void> => new Promise((r) => setTimeout(r, 0));
