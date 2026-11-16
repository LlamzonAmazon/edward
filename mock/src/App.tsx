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
  deactivate as postDeactivate,
  uninstall as postUninstall,
  type FindingState,
  type Status,
  type Store,
} from "./api";
import { agentNames, filesPrompt, Finding, Source } from "./data";
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

export default function App() {
  const [store, setStore] = useState(fixtureStore);
  // Nothing renders until the first store read finishes, so a real install never
  // shows the fixture graph for a frame. The mock falls back to fixtures when no
  // API answers.
  const [booted, setBooted] = useState(false);
  const [entering, setEntering] = useState(false);
  const [firstRunOpen, setFirstRunOpen] = useState(false);
  // During first run the map and sidebar show an empty Mac. Closing first run
  // (Done, Skip or close) reveals the data: dots leave their sources, counts
  // count up, and the sidebar fills (cascade).
  const [revealed, setRevealed] = useState(false);
  const [cascade, setCascade] = useState(false);
  const [sourceId, setSourceId] = useState<string | null>(null);
  const [rowId, setRowId] = useState<string | null>(null);
  const [findingId, setFindingId] = useState<string | null>(null);
  const [allOpen, setAllOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  // First run starts with no agent picked. Each row still says where it was found.
  const [agentIds, setAgentIds] = useState<string[]>([]);
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

  // The page's one entrance: the graph builds outward across --dur-stagger
  // (components.css, "First load"), and first run opens once it has finished.
  const boot = useCallback((firstRun: boolean) => {
    setBooted(true);
    setRevealed(!firstRun);
    setEntering(true);
    const built = token("--dur-stagger");
    window.setTimeout(() => setEntering(false), built + token("--dur-slow"));
    if (firstRun) window.setTimeout(() => setFirstRunOpen(true), built);
  }, []);

  useEffect(() => {
    let cancelled = false;
    getStore()
      .then((s) => {
        if (cancelled) return;
        live.current = true;
        applyStore(s, { tick: true });
        if (!s.firstRunDone) setAgentIds(s.hookedAgents);
        boot(!s.firstRunDone);
      })
      .catch(() => {
        if (cancelled) return;
        logFallback();
        boot(true);
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

  const surface = useMemo(
    () => makeSurface(revealed ? store : { ...store, sources: store.sources.map((s) => ({ ...s, items: [] })) }, setFilesOn),
    [store, setFilesOn, revealed],
  );
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

  const revealedRef = useRef(false);
  revealedRef.current = revealedRef.current || revealed;
  const closeFinding = useCallback(() => setFindingId(null), []);
  const closeSource = useCallback(() => setSourceId(null), []);
  const closeFirstRun = useCallback(() => {
    if (!storeRef.current.firstRunDone) {
      write(() => postFirstRun(true), (s) => ({ ...s, firstRunDone: true }));
    }
    setFirstRunOpen(false);
    if (revealedRef.current) return;
    revealedRef.current = true;
    // Start once the dialog has finished closing, and keep the cascade timing
    // until the last source and the sidebar have arrived.
    window.setTimeout(() => {
      setRevealed(true);
      setCascade(true);
      window.setTimeout(() => setCascade(false), token("--dur-stagger") * 2 + token("--dur-reveal"));
    }, token("--dur-slow"));
  }, [write]);
  // Turn off and Delete stop the program that serves this page. It replies,
  // quits about 500 ms later, and its LaunchAgent is removed about a second
  // after that (launchd may start it once more in between). So success means
  // the API has stopped answering, checked until a deadline. A reply with an
  // HTTP error means EDwaRd refused, and is reported at once.
  const stopEdward = useCallback(async (post: () => Promise<unknown>): Promise<"ok" | "fail"> => {
    if (!live.current) return "fail";
    try {
      await post();
    } catch (e) {
      if (!(e instanceof TypeError)) return "fail";
    }
    const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
    const deadline = performance.now() + 8000;
    await wait(1500);
    while (performance.now() < deadline) {
      try {
        await getStore();
      } catch {
        live.current = false;
        return "ok";
      }
      await wait(300);
    }
    return "fail";
  }, []);
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

  if (!booted) return null;

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
          cascade={cascade}
        />
        <main className="main">
          <Graph entering={entering} cascade={cascade} state={state} onSource={(s) => openSource(s)} onFinding={(f) => setFindingId(f.id)} onItem={(s, id) => openSource(s, id)} />
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
          paused={store.paused}
          onPause={() => {
            const paused = !storeRef.current.paused;
            write(() => postPause(paused), (s) => ({ ...s, paused }));
          }}
          onDeactivate={() => stopEdward(postDeactivate)}
          onUninstall={() => stopEdward(postUninstall)}
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
