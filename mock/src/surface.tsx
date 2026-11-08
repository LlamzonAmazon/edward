import { createContext, useContext } from "react";
import { fixtureStore, type Host, type Store } from "./api";
import type { Agent, Finding, Link, Source } from "./data";

export type Surface = {
  sources: Source[];
  findings: Finding[];
  links: Link[];
  filesOn: boolean;
  setFilesOn: (on: boolean) => void;
  host: Host;
  agents: Agent[];
};

export function makeSurface(store: Store, setFilesOn: (on: boolean) => void): Surface {
  const sources = store.sources.map((s) => (s.optIn && !store.filesOn ? { ...s, collected: false, items: [] } : s));
  const present = new Set(sources.flatMap((s) => s.items.map((i) => i.id)));
  return {
    sources,
    findings: store.findings.filter((f) => present.has(f.itemId)),
    links: store.links.filter((l) => present.has(l.a) && present.has(l.b)),
    filesOn: store.filesOn,
    setFilesOn,
    host: store.host,
    agents: store.agents,
  };
}

export const SurfaceContext = createContext<Surface>(makeSurface(fixtureStore(), () => {}));

export const useSurface = () => useContext(SurfaceContext);
