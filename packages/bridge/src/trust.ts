// Which pages get the bridge: the rule of isTrusted in Go (window.go), which
// the core sends as the `trust` of the bridge configuration, so that the
// page decides for itself at document start, before any of its scripts run.

/** The rule of isTrusted, as data. */
export interface Trust {
  /** Schemes whose pages are trusted, whatever their host. */
  schemes: string[];
  /** Lower case origins, scheme://host[:port], of trusted pages; "*" trusts every page. */
  origins: string[];
  /** Pages of http and https on localhost, 127.0.0.1 and ::1 are trusted. */
  loopback: boolean;
}

/** Reports whether the page at loc is trusted. */
export function isTrusted(loc: Pick<Location, "protocol" | "host" | "hostname">, trust: Trust): boolean {
  const scheme = loc.protocol.replace(/:$/, "").toLowerCase();
  if (trust.schemes.includes(scheme)) return true;
  if (trust.origins.includes("*") || trust.origins.includes(`${scheme}://${loc.host.toLowerCase()}`)) return true;
  return (
    trust.loopback &&
    (scheme === "http" || scheme === "https") &&
    ["localhost", "127.0.0.1", "::1"].includes(loc.hostname.toLowerCase().replace(/^\[(.*)\]$/, "$1"))
  );
}
