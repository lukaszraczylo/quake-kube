export const DEFAULT_CVARS: ReadonlyArray<readonly [string, string]> = [
  ["cl_allowDownload", "1"],
  ["cl_timeout", "120"],
  ["rate", "100000"],
  ["snaps", "40"],
  ["cl_maxpackets", "125"],
  ["cl_packetdup", "0"],
  ["com_maxfps", "250"],
  ["r_customwidth", "4096"],
  ["r_customheight", "2160"],
  ["cg_fov", "105"],
  ["cg_drawfps", "0"],
  ["r_picmip", "0"],
  ["r_lodbias", "-2"],
  ["r_hdr", "1"],
  ["r_dynamiclight", "1"],
  ["r_shadowFilter", "2"],
  ["r_flares", "1"],
  ["r_subdivisions", "1"],
  ["r_colorbits", "32"],
  ["r_texturebits", "32"],
  ["r_ext_max_anisotropy", "16"],
  ["r_ext_multisample", "2"],
  ["r_ext_framebuffer_multisample", "2"],
  ["r_ext_compressed_textures", "1"],
  ["r_textureMode", "GL_LINEAR_MIPMAP_LINEAR"],
  ["r_finish", "0"],
  ["r_swapInterval", "0"],
  ["r_deluxeMapping", "1"],
  ["com_hunkmegs", "96"],
  ["cl_timenudge", "0"],
  ["cg_predictItems", "1"],
  ["net_socksEnabled", "0"],
  ["com_zoneMegs", "24"],
];

/** Turns `?a b&c d` into engine commands `+a b +c d`. */
export function queryCommands(search: string): string[] {
  const query = search.startsWith("?") ? search.slice(1) : search;
  const args: string[] = [];
  for (const part of query.match(/[^&=]+/g) ?? []) {
    const words = decodeURIComponent(part).split(" ");
    words[0] = "+" + words[0];
    args.push(...words);
  }
  return args;
}

/** Adds the port the engine needs when the page host has none. */
export function connectHost(host: string, protocol: string): string {
  if (host.includes(":")) return host;
  return host + (protocol === "https:" ? ":443" : ":80");
}

export interface JoinOptions {
  name: string;
  host: string;
  search: string;
  password?: string;
}

export function buildArgs(o: JoinOptions): string[] {
  const args = ["+set", "name", o.name, "+name", o.name, "+set", "fs_cdn", o.host, "+connect", o.host];
  for (const [key, value] of DEFAULT_CVARS) args.push("+set", key, value);
  args.push(...queryCommands(o.search));
  if (o.password) args.push("+set", "password", o.password);
  return args;
}
