import { useEffect, useState } from "react";
import { agentNames, categories, Finding } from "./data";
import { Icon, Swap, useHold, Wipe } from "./ui";
import { AccessList, AgentPicks, Device } from "./Setup";
import { useSurface } from "./surface";
import type { FindingState } from "./App";

type View = "main" | "agent" | "findings";

type Props = {
  open: boolean;
  agentIds: string[];
  onAgents: (ids: string[]) => void;
  context: string;
  onContext: (c: string) => void;
  state: Record<string, FindingState>;
  onNote: (id: string, note: string | undefined) => void;
  paused: boolean;
  onPause: () => void;
  onDeactivate: () => Promise<"ok" | "fail">;
  onUninstall: () => Promise<"ok" | "fail">;
  onClose: () => void;
};

type StopKind = "pause" | "off" | "delete" | null;
type StopPhase = "pick" | "confirm" | "working" | "done" | "fail";

const titles: Record<View, string> = {
  main: "Settings",
  agent: "Notes for your agent",
  findings: "Notes on findings",
};

export function Settings(props: Props) {
  const { open, agentIds, onAgents, context, state, paused, onPause, onDeactivate, onUninstall, onClose } = props;
  const { findings } = useSurface();
  const [view, setView] = useState<View>("main");
  const [dir, setDir] = useState<"next" | "back">("next");
  const [stopKind, setStopKind] = useState<StopKind>(null);
  const [stopPhase, setStopPhase] = useState<StopPhase>("pick");
  const [stopMsg, setStopMsg] = useState("");
  useEffect(() => {
    if (open) {
      setView("main");
      setStopKind(null);
      setStopPhase("pick");
      setStopMsg("");
    }
  }, [open]);
  const go = (v: View) => {
    setDir(v === "main" ? "back" : "next");
    setView(v);
    setStopKind(null);
    setStopPhase("pick");
    setStopMsg("");
  };
  const noted = findings.filter((f) => state[f.id]?.note);
  const firstLine = context.split("\n")[0];

  const choose = (kind: StopKind) => {
    setStopKind(kind);
    setStopPhase("confirm");
    setStopMsg("");
  };

  const runStop = async () => {
    if (!stopKind || stopKind === "pause") return;
    setStopPhase("working");
    setStopMsg(stopKind === "off" ? "Turning EDwaRd off…" : "Deleting EDwaRd from this Mac…");
    const result = stopKind === "off" ? await onDeactivate() : await onUninstall();
    if (result === "ok") {
      setStopPhase("done");
      setStopMsg(
        stopKind === "off"
          ? "EDwaRd is off. It will not start at login. Your store is still on disk. Close this tab. To start again: make mock && make app && sh install.sh"
          : "EDwaRd is deleted from this Mac. Close this tab. The page will not load again until you install.",
      );
      return;
    }
    setStopPhase("fail");
    setStopMsg(stopKind === "off" ? "Could not turn EDwaRd off. Is the agent running?" : "Could not delete EDwaRd. Is the agent running?");
  };

  return (
    <>
      <div className="dialog__head">
        {view === "main" ? (
          <span className="icon-box">
            <Icon name="settings" />
          </span>
        ) : (
          <button className="icon-btn" onClick={() => go("main")} aria-label="Back to settings">
            <Icon name="back" />
          </button>
        )}
        <h2 className="t-title grow">
          <Swap swapKey={view}>{titles[view]}</Swap>
        </h2>
        <button className="icon-btn" onClick={onClose} aria-label="Close" disabled={stopPhase === "working"}>
          <Icon name="close" />
        </button>
      </div>
      <div className="dialog__body">
        <div className="step stack stack--loose" key={view} data-dir={dir}>
          {view === "main" && (
            <>
              <div className="section">
                <span className="t-head">Notes</span>
                <div className="list">
                  <Entry icon="agent" title="Notes for your agent" detail={firstLine || "None yet"} onClick={() => go("agent")} />
                  <Entry
                    icon="check"
                    title="Notes on findings"
                    detail={noted.length ? noted.map((f) => f.value).join(", ") : "None yet. Save one from any finding."}
                    meta={noted.length ? String(noted.length) : undefined}
                    onClick={() => go("findings")}
                  />
                </div>
              </div>
              <div className="section">
                <span className="t-head">Your agents</span>
                <AgentPicks agentIds={agentIds} onAgents={onAgents} />
              </div>
              <div className="section">
                <span className="t-head">What EDwaRd can see</span>
                <Device />
                <AccessList />
              </div>
              <div className="section">
                <span className="t-head">Pause, turn off, or delete</span>
                <div className="step stack" key={`${stopPhase}-${stopKind ?? "none"}`} data-dir="next">
                  {stopPhase === "pick" && (
                    <>
                      <p className="muted">Three different actions. Pause keeps the program running. Turn off quits it. Delete removes it.</p>
                      <div className="list">
                        <StopChoice
                          title={paused ? "Resume looking" : "Pause looking"}
                          detail={paused ? "Collectors are stopped. Click to start them again." : "Stop collectors only. The program stays running. Same as Looking / Paused in the sidebar."}
                          action={paused ? "Resume" : "Pause"}
                          reserve={["Resume", "Pause"]}
                          swapKey={paused ? "paused" : "looking"}
                          onClick={() => {
                            onPause();
                            setStopMsg(paused ? "Looking again." : "Paused. Collectors are stopped until you resume.");
                          }}
                        />
                        <StopChoice title="Turn off EDwaRd" detail="Quit the program and remove the login item. Store and skills stay on disk. Install again to start." action="Turn off" onClick={() => choose("off")} />
                        <StopChoice title="Delete EDwaRd from this Mac" detail="Remove the program, store, logs, login item, PATH link, and EDwaRd skill files. Same as edward uninstall." action="Delete" onClick={() => choose("delete")} />
                      </div>
                      {stopMsg ? (
                        <p className="t-strong step" key={stopMsg} data-dir="next">
                          {stopMsg}
                        </p>
                      ) : null}
                    </>
                  )}
                  {stopPhase === "confirm" && stopKind === "off" && (
                    <div className="stack">
                      <p className="t-strong">Turn off EDwaRd?</p>
                      <p className="muted">The program quits now and will not start at login. Your findings store stays. Skills stay. To run again you install from the repo.</p>
                      <div className="picks">
                        <Wipe tone="flag" onClick={runStop}>
                          Yes, turn off
                        </Wipe>
                        <Wipe tone="quiet" onClick={() => setStopPhase("pick")}>
                          Cancel
                        </Wipe>
                      </div>
                    </div>
                  )}
                  {stopPhase === "confirm" && stopKind === "delete" && (
                    <div className="stack">
                      <p className="t-strong">Delete EDwaRd from this Mac?</p>
                      <p className="muted">This removes the program, the store, the login item, the PATH link, and the EDwaRd skill files. It does not undo macOS privacy grants.</p>
                      <div className="picks">
                        <Wipe tone="flag" onClick={runStop}>
                          Yes, delete
                        </Wipe>
                        <Wipe tone="quiet" onClick={() => setStopPhase("pick")}>
                          Cancel
                        </Wipe>
                      </div>
                    </div>
                  )}
                  {(stopPhase === "working" || stopPhase === "done" || stopPhase === "fail") && (
                    <div className="stack">
                      <p className="t-strong">
                        <Swap swapKey={stopPhase}>{stopPhase === "working" ? "Working…" : stopPhase === "done" ? "Done" : "Did not finish"}</Swap>
                      </p>
                      <p className="muted">
                        <Swap block swapKey={stopMsg}>
                          {stopMsg}
                        </Swap>
                      </p>
                      {stopPhase !== "working" && (
                        <div className="picks">
                          <Wipe tone="accent" onClick={onClose}>
                            Close
                          </Wipe>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </>
          )}
          {view === "agent" && <AgentNotes {...props} />}
          {view === "findings" &&
            (noted.length ? (
              <div className="list">
                {noted.map((f) => (
                  <FindingNote key={f.id} finding={f} note={state[f.id].note!} names={agentNames(agentIds)} onNote={(n) => props.onNote(f.id, n)} />
                ))}
              </div>
            ) : (
              <p className="muted">No notes yet. Open a finding and save one, and it shows up here.</p>
            ))}
        </div>
      </div>
    </>
  );
}

function StopChoice({ title, detail, action, swapKey, reserve, onClick }: { title: string; detail: string; action: string; swapKey?: string; reserve?: string[]; onClick: () => void }) {
  return (
    <div className="row">
      <div className="row__head" style={{ cursor: "default" }}>
        <span className="row__text">
          <span className="row__title">
            <Swap swapKey={swapKey ?? title}>{title}</Swap>
          </span>
          <span className="row__detail">
            <Swap block swapKey={swapKey ?? detail}>
              {detail}
            </Swap>
          </span>
        </span>
        <Wipe tone="quiet" size="s" swapKey={swapKey ?? action} reserve={reserve} onClick={onClick}>
          {action}
        </Wipe>
      </div>
    </div>
  );
}

function Entry({ icon, title, detail, meta, onClick }: { icon: "agent" | "check"; title: string; detail: string; meta?: string; onClick: () => void }) {
  return (
    <div className="row">
      <button className="row__head" onClick={onClick}>
        <span className="icon-box">
          <Icon name={icon} />
        </span>
        <span className="row__text">
          <span className="row__title">{title}</span>
          <span className="row__detail">{detail}</span>
        </span>
        {meta && <span className="row__meta">{meta}</span>}
        <Icon name="arrow" />
      </button>
    </div>
  );
}

function AgentNotes({ agentIds, context, onContext }: Props) {
  const [draft, setDraft] = useState(context);
  useEffect(() => setDraft(context), [context]);
  const changed = draft.trim() !== context.trim();
  const [saved, showSaved] = useHold<"ok">();
  return (
    <form
      className="section"
      onSubmit={(e) => {
        e.preventDefault();
        if (!changed) return;
        onContext(draft.trim());
        showSaved("ok");
      }}
    >
      <p className="muted">Your agent reads these before anything EDwaRd raised. They never change what the page raises.</p>
      <textarea
        id="context"
        className="field__input field__textarea field__textarea--tall"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        placeholder={"Port 3000 is always my dev server.\nIgnore anything in ~/scratch."}
      />
      <div>
        <Wipe tone="accent" swapKey="save" disabled={!changed && !saved} done={saved} doneLabel={`Saved for ${agentNames(agentIds)}`}>
          <Icon name="arrow" />
          {`Save for ${agentNames(agentIds)}`}
        </Wipe>
      </div>
    </form>
  );
}

function FindingNote({ finding, note, names, onNote }: { finding: Finding; note: string; names: string; onNote: (n: string | undefined) => void }) {
  const cat = categories[finding.category];
  const [draft, setDraft] = useState(note);
  useEffect(() => setDraft(note), [note]);
  const changed = draft.trim() !== note && !!draft.trim();
  const [saved, showSaved] = useHold<"ok">();
  return (
    <form
      className="finding-note"
      onSubmit={(e) => {
        e.preventDefault();
        if (!changed) return;
        onNote(draft.trim().replace(/\.$/, ""));
        showSaved("ok");
      }}
    >
      <span className="finding-note__head">
        <span className="cat-tile cat-tile--s" data-cat={cat.color}>
          <Icon name={cat.icon} />
        </span>
        <span className="t-strong">{cat.name}</span>
        <span className="muted">{finding.value}</span>
      </span>
      <input className="field__input" value={draft} onChange={(e) => setDraft(e.target.value)} aria-label={`Note on ${finding.value}`} />
      <span className="picks">
        <Wipe tone={changed || saved ? "accent" : "quiet"} size="s" disabled={!changed && !saved} done={saved} doneLabel="Saved">
          <Icon name="arrow" />
          Save note for {names}
        </Wipe>
        <Wipe tone="quiet" size="s" onClick={() => onNote(undefined)}>
          <Icon name="close" />
          Remove
        </Wipe>
      </span>
    </form>
  );
}
