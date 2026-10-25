const KEY = "playerName";
const PLACEHOLDER = "enter player name";
function read() {
    try {
        return localStorage.getItem(KEY);
    }
    catch {
        return null;
    }
}
function write(name) {
    try {
        localStorage.setItem(KEY, name);
    }
    catch {
        // storage blocked: the name only lasts for this session
    }
}
export function storedName() {
    return read();
}
export function placeholder() {
    return read() ?? PLACEHOLDER;
}
/** Picks the typed name, else the stored one, else a random default; remembers the result. */
export function resolveName(typed) {
    const name = typed.trim() || read() || `Player_${Math.floor(Math.random() * 1000)}`;
    write(name);
    return name;
}
