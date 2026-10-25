# Upstream ioq3 WASM client spike

Experiment: replace the 2014 QuakeJS fork (asm.js, `public/ioquake3.js`) with the upstream
`ioquake/ioq3` Emscripten target. Nothing under `public/` changes on this branch.

## Result (2026-10-25, ioq3 `87a99d1`, emsdk 6.0.13)

- Builds with `engine/build.sh`: `ioquake3.wasm` 2.3 MB + `ioquake3.js` 236 KB (the current asm.js file is 3.9 MB).
- Joins the existing server through the unchanged Go WebSocket proxy. Emscripten SOCKFS sends the same
  `\xff\xff\xff\xffport` handshake that `internal/quake/client/proxy.go` expects.
- In game on a local Docker stack: 60.0 fps, p99 17.7 ms, max 17.8 ms, 0 frames over 25 ms (one client, 15 s sample).

## Client settings that were required

- `-DBUILD_STANDALONE=ON`, otherwise the engine rejects the pak0 checksum.
- `+set com_gamename Quake3Arena`, otherwise the server answers "Game mismatch".
- `+set net_enabled 1`; the upstream demo page sets `net_enabled 0`.
- `Module.websocket.url` pointing at the proxy.
- `pak0.pk3` to `pak8.pk3` in the virtual file system. pak0 alone ships old demo QVMs ("User Interface is version 3, expected 6"); the server's `qagame.qvm` comes from `pak8`.

## Not done (needed before this can replace the current client)

- No HTTP downloader in the WASM build (`USE_HTTP OFF`). The client must fetch the pk3 list itself, as `UpdateManifest`/`SyncPaks` do today, and cache it in IndexedDB.
- The 175 MB `pak0.pk3` is fetched whole on every load in this spike. It needs the IndexedDB cache.
- The `web/` TypeScript layer (join form, name, EULA prompt, loading label, pointer lock, resize) targets the old `Module`/`SYS` globals and needs a port.
- Only one client was tested. No bots-map rotation, no reconnect, no fullscreen, no audio check.

## Try it

    engine/build.sh && cp engine/test.html <ioq3>/build-em/Release/
    # put pak0..pak8.pk3 in build-em/Release/baseq3, run the q3 server image with --client-addr, then:
    python3 -m http.server 18098 --directory <ioq3>/build-em/Release
    # open http://localhost:18098/test.html?ws=ws://127.0.0.1:<proxy-port>/&connect=127.0.0.1:27960
