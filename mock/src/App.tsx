import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  files as postFiles,
  findingStatus,
  firstRun as postFirstRun,
  fixtureStore,
  getStore,
  hook as postHook,
  note as postNote,
  notes as postNotes,
  pause as postPause,
  type FindingState,
  type Status,
  type Store,
} from "./api";
import { agentNames, agents, filesPrompt, Finding, Source } from "./data";
import { makeSurface, SurfaceContext } from "./surface";
import { Scrim, Toast, token, useEscape, useLatched } from "./ui";
import { Graph } from "./Graph";
import { Nav } from "./Nav";
import { Shelf } from "./Shelf";
import { FindingView } from "./FindingView";
import { FirstRun } from "./FirstRun";
import { AllRaised } from "./AllRaised";
import { Settings } from "./Settings";

export type { FindingState, Status };

const POLL_SECONDS = 5;

let fallbackLogged = false;
const logFallback = () => {
  if (fallbackLogged) return;
  fallbackLogged = true;
  console.info("EDwaRd: store API unavailable, using fixture data");
};

const foundIds = (s: Store) => s.agents.filter((a) => a.found).map((a) => a.id);

export default function App() {
  const [store, setStore] = useState(fixtureStore);
  const [firstRunOpen, setFirstRunOpen] = useState(true);
  const [sourceId, setSourceId] = useState<string | null>(null);
  const [rowId, setRowId] = useState<string | null>(null);
  const [findingId, setFindingId] = useState<string | null>(null);
  const [allOpen, setAllOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [agentIds, setAgentIds] = useState(() => agents.filter((a) => a.found).map((a) => a.id));
  const [toast, setToast] = useState<{ msg: string; undo?: () => void } | null>(null);
  const [age, setAge] = useState(2);

  const live = useRef(false);
  const storeRef = useRef(store);
  storeRef.current = store;
  const agentIdsRef = useRef(agentIds);
  agentIdsRef.current = agentIds;

  const applyStore = useCallback((s: Store, opts?: { tick?: boolean }) => {
    setStore(s);
    storeRef.current = s;
    if (opts?.tick) setAge(0);
    if (s.firstRunDone) setAgentIds(s.hookedAgents);
  }, []);

  const write = useCallback(
    (fn: () => Promise<Store>, optimistic: (s: Store) => Store) => {
      const next = optimistic(storeRef.current);
      setStore(next);
      storeRef.current = next;
      if (!live.current) return;
      fn().then(applyStore).catch(() => {});
    },
    [applyStore],
  );

  useEffect(() => {
    let cancelled = false;
    getStore()
      .then((s) => {
        if (cancelled) return;
        live.current = true;
        applyStore(s, { tick: true });
        if (s.firstRunDone) setFirstRunOpen(false);
        else setAgentIds(s.hookedAgents.length ? s.hookedAgents : foundIds(s));
      })
      .catch(() => {
        if (!cancelled) logFallback();
      });
    return () => {
      cancelled = true;
    };
  }, [applyStore]);

  useEffect(() => {
    if (store.paused) return;
    const t = window.setInterval(() => setAge((a) => (a + 1) % POLL_SECONDS), 1000);
    return () => window.clearInterval(t);
  }, [store.paused]);

  useEffect(() => {
    if (store.paused) return;
    const t = window.setInterval(() => {
      if (!live.current) return;
      getStore()
        .then((s) => applyStore(s, { tick: true }))
        .catch(() => {});
    }, POLL_SECONDS * 1000);
    return () => window.clearInterval(t);
  }, [store.paused, applyStore]);

  useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), token("--dur-toast"));
    return () => window.clearTimeout(t);
  }, [toast]);

  const setFilesOn = useCallback(
    (on: boolean) => {
      if (live.current) {
        postFiles(on)
          .then((s) => {
            applyStore(s);
            if (on && s.filesGrant === "denied") setToast({ msg: filesPrompt });
            else setToast({ msg: s.filesOn ? "New files is on. macOS asks you once." : "New files is off." });
          })
          .catch(() => {
            setStore((s) => ({ ...s, filesOn: on }));
            setToast({ msg: on ? "New files is on. macOS asks you once." : "New files is off." });
          });
        return;
      }
      setStore((s) => ({ ...s, filesOn: on, filesGrant: on ? "allowed" : s.filesGrant }));
      setToast({ msg: on ? "New files is on. macOS asks you once." : "New files is off." });
    },
    [applyStore],
  );

  const surface = useMemo(() => makeSurface(store, setFilesOn), [store, setFilesOn]);
  const { sources, findings } = surface;
  const state = useMemo(() => {
    const next: Record<string, FindingState> = { ...store.states };
    for (const f of findings) {
      if (!next[f.id]) next[f.id] = { status: "raised" };
    }
    for (const s of sources) {
      for (const it of s.items) {
        if (it.findingId && !next[it.findingId]) next[it.findingId] = { status: "raised" };
      }
    }
    return next;
  }, [store.states, findings, sources]);
  const stateRef = useRef(state);
  stateRef.current = state;

  const source = useLatched(sources.find((s) => s.id === sourceId) ?? null);
  const finding = useLatched(findings.find((f) => f.id === findingId) ?? null);

  const closeFinding = useCallback(() => setFindingId(null), []);
  const closeSource = useCallback(() => setSourceId(null), []);
  const closeFirstRun = useCallback(() => {
    if (!storeRef.current.firstRunDone) {
      write(() => postFirstRun(true), (s) => ({ ...s, firstRunDone: true }));
    }
    setFirstRunOpen(false);
  }, [write]);
  const closeAll = useCallback(() => setAllOpen(false), []);
  const closeSettings = useCallback(() => setSettingsOpen(false), []);
  const top = findingId ? closeFinding : settingsOpen ? closeSettings : allOpen ? closeAll : sourceId ? closeSource : closeFirstRun;
  useEscape(top, !!(findingId || settingsOpen || allOpen || sourceId || firstRunOpen));

  const patch = useCallback(
    (id: string, p: Partial<FindingState>) => {
      const before = stateRef.current[id] ?? { status: "raised" as const };
      const merged: FindingState = { ...before, ...p };
      if ("note" in p && !p.note) delete merged.note;
      write(
        async () => {
          let latest = storeRef.current;
          if (p.status) latest = await findingStatus(id, p.status);
          if ("note" in p) latest = await postNote(id, p.note ?? null);
          return latest;
        },
        (s) => ({ ...s, states: { ...s.states, [id]: merged } }),
      );
    },
    [write],
  );

  const openSource = (s: Source, row: string | null = null) => {
    setRowId(row);
    setSourceId(s.id);
  };

  const clear = (f: Finding) => {
    const before = state[f.id];
    patch(f.id, { status: "cleared" });
    setFindingId(null);
    setToast({
      msg: `Cleared. EDwaRd won’t raise ${f.value} again.`,
      undo: () => {
        patch(f.id, before);
        setToast(null);
      },
    });
  };

  const ask = (f: Finding) => {
    patch(f.id, { status: "asked" });
    setToast({ msg: `Copied. Paste it into ${agentNames(agentIds, " or ")}.` });
  };

  const readAgo = store.paused ? "Paused" : age === 0 ? "Just now" : `${age} s ago`;
  const dialogOpen = !!findingId || allOpen || settingsOpen || firstRunOpen;

  return (
    <SurfaceContext.Provider value={surface}>
      <div className="app">
        <Nav
          state={state}
          activeFinding={findingId}
          activeSource={sourceId}
          onFinding={(f) => setFindingId(f.id)}
          onSource={(s) => openSource(s)}
          paused={store.paused}
          tick={!store.paused && age === 0}
          readAgo={readAgo}
          onPause={() => {
            const paused = !storeRef.current.paused;
            write(() => postPause(paused), (s) => ({ ...s, paused }));
          }}
          onAll={() => setAllOpen(true)}
          onSettings={() => setSettingsOpen(true)}
          settingsOpen={settingsOpen}
        />
        <main className="main">
          <Graph state={state} onSource={(s) => openSource(s)} onFinding={(f) => setFindingId(f.id)} onItem={(s, id) => openSource(s, id)} />
        </main>
      </div>

      <Scrim open={!!sourceId} onClose={closeSource} />
      <aside className="sheet" data-open={!!sourceId} aria-hidden={!sourceId}>
        {source && <Shelf source={source} state={state} readAgo={readAgo} initialRow={rowId} onFinding={(f) => setFindingId(f.id)} onClose={closeSource} />}
      </aside>

      <Scrim open={dialogOpen} onClose={top} level={2} />
      <div className="dialog" data-size="m" data-open={allOpen && !findingId} role="dialog" aria-hidden={!allOpen || !!findingId}>
        <AllRaised state={state} onFinding={(f) => setFindingId(f.id)} onClose={closeAll} />
      </div>

      <div className="dialog" data-size="fixed" data-open={settingsOpen} role="dialog" aria-hidden={!settingsOpen}>
        <Settings
          open={settingsOpen}
          state={state}
          onNote={(id, n) => patch(id, { note: n })}
          agentIds={agentIds}
          onAgents={(ids) => {
            setAgentIds(ids);
            if (live.current && storeRef.current.firstRunDone) {
              postHook(ids).then(applyStore).catch(() => {});
            }
          }}
          context={store.agentNotes}
          onContext={(c) => {
            write(() => postNotes(c), (s) => ({ ...s, agentNotes: c }));
            setToast({ msg: `Saved for ${agentNames(agentIds)}.` });
          }}
          onClose={closeSettings}
        />
      </div>

      <div className="dialog" data-open={!!findingId} role="dialog" aria-hidden={!findingId}>
        {finding && (
          <FindingView
            key={finding.id}
            finding={finding}
            st={state[finding.id]}
            agentNames={agentNames(agentIds)}
            onClose={closeFinding}
            onClear={() => clear(finding)}
            onAsk={() => ask(finding)}
            onNote={(n) => patch(finding.id, { note: n })}
          />
        )}
      </div>

      <div className="dialog" data-size="fixed" data-open={firstRunOpen} role="dialog" aria-hidden={!firstRunOpen}>
        <FirstRun
          agentIds={agentIds}
          onAgents={setAgentIds}
          onClose={closeFirstRun}
          onWritten={() => {
            const ids = agentIdsRef.current;
            write(
              async () => {
                await postHook(ids);
                return postFirstRun(true);
              },
              (s) => ({ ...s, firstRunDone: true, hookedAgents: ids }),
            );
            setToast({ msg: `Added EDwaRd to ${agentNames(ids, " and ")}.` });
          }}
        />
      </div>

      <Toast open={!!toast} message={toast?.msg} action={toast?.undo ? "Undo" : undefined} onAction={toast?.undo} />
    </SurfaceContext.Provider>
  );
}
