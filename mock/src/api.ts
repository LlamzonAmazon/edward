import { agents, findings, host, links, sources, type Agent, type Finding, type Link, type Source } from "./data";

export type Status = "raised" | "cleared" | "asked";
export type FindingState = { status: Status; note?: string };
export type FilesGrant = "off" | "prompting" | "allowed" | "denied";
export type Host = { name: string; os: string; user: string };
export type Dismissal = { key: string };
export type Baseline = { startupIds: string[]; fileIds: string[] };

export type Store = {
  version: 1;
  writtenAt: string;
  paused: boolean;
  filesOn: boolean;
  filesGrant: FilesGrant;
  firstRunDone: boolean;
  host: Host;
  agentNotes: string;
  hookedAgents: string[];
  agents: Agent[];
  sources: Source[];
  findings: Finding[];
  states: Record<string, FindingState>;
  dismissals: Dismissal[];
  links: Link[];
  baseline: Baseline;
};

export function fixtureStore(): Store {
  return {
    version: 1,
    writtenAt: "2026-11-07T00:00:00Z",
    paused: false,
    filesOn: false,
    filesGrant: "off",
    firstRunDone: false,
    host,
    agentNotes: "",
    hookedAgents: [],
    agents,
    sources,
    findings,
    states: Object.fromEntries(findings.map((f) => [f.id, { status: "raised" as const }])),
    dismissals: [],
    links,
    baseline: { startupIds: [], fileIds: [] },
  };
}

async function readJson<T>(res: Response): Promise<T> {
  if (!res.ok) throw new Error(`${res.status} ${res.url}`);
  return (await res.json()) as T;
}

export function getStore(): Promise<Store> {
  return fetch("/api/store", { cache: "no-store" }).then((res) => readJson<Store>(res));
}

function post<T>(path: string, body: unknown): Promise<T> {
  return fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  }).then((res) => readJson<T>(res));
}

export const pause = (paused: boolean) => post<Store>("/api/pause", { paused });
export const files = (on: boolean) => post<Store>("/api/files", { on });
export const findingStatus = (id: string, status: Status) =>
  post<Store>(`/api/findings/${encodeURIComponent(id)}/status`, { status });
export const note = (id: string, text: string | null) =>
  post<Store>(`/api/findings/${encodeURIComponent(id)}/note`, { note: text });
export const notes = (text: string) => post<Store>("/api/notes", { text });
export const hook = (agentIds: string[]) => post<Store>("/api/hook", { agentIds });
export const firstRun = (done: boolean) => post<Store>("/api/first-run", { done });
export const openPrivacy = (pane: "files" | "fda") => post<{ ok: true }>("/api/open-privacy", { pane });
