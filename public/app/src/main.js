import { buildArgs, connectHost } from "./args.js";
import { placeholder, resolveName, storedName } from "./player.js";
import { showError, showLoading } from "./ui.js";
const WASM_MEMORY = 384 * 1024 * 1024;
// ioquake3.js reads Module when it starts, and both scripts are deferred in
// document order, so this assignment runs first.
window.Module = {
    TOTAL_MEMORY: WASM_MEMORY,
    canvas: null,
    printErr: (text) => console.error(text),
    setStatus: (text) => console.log(text),
};
function fullscreenActive() {
    const d = document;
    return ["webkitFullScreenElement", "webkitFullscreenElement", "mozFullScreenElement",
        "mozFullscreenElement", "fullScreenElement", "fullscreenElement"].some((k) => d[k]);
}
function byId(id) {
    return document.getElementById(id);
}
function join() {
    const nameInput = byId("playerName");
    const frame = byId("viewport-frame");
    const main = byId("main");
    if (!nameInput || !frame)
        return;
    const args = buildArgs({
        name: resolveName(nameInput.value),
        host: connectHost(document.location.host, window.location.protocol),
        search: window.location.search,
        password: byId("password")?.value,
    });
    main?.remove();
    showLoading(frame);
    try {
        ioq3.callMain(args);
    }
    catch (err) {
        showError(err);
    }
}
function initForm() {
    const nameInput = byId("playerName");
    if (nameInput) {
        nameInput.placeholder = placeholder();
        nameInput.value = storedName() ?? "";
    }
    byId("join-form")?.addEventListener("submit", (e) => {
        e.preventDefault();
        join();
    });
}
function initEngine() {
    const frame = byId("viewport-frame");
    ioq3.viewport = frame;
    ioq3.elementPointerLock = true;
    ioq3.exitHandler = (err) => {
        if (err) {
            showError(err);
            return;
        }
        window.location.href = "/";
    };
    window.addEventListener("resize", () => {
        if (!ioq3.canvas || !ioq3.viewport || fullscreenActive())
            return;
        ioq3.setCanvasSize(ioq3.viewport.offsetWidth, ioq3.viewport.offsetHeight);
    });
}
initForm();
window.addEventListener("load", initEngine);
