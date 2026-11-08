import { useEffect, useState } from "react";
import { agentNames, categories, Finding } from "./data";
import { Icon, Wipe } from "./ui";
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
  onClose: () => void;
};

const titles: Record<View, string> = {
  main: "Settings",
  agent: "Notes for your agent",
  findings: "Notes on findings",
};

export function Settings(props: Props) {
  const { open, agentIds, onAgents, context, state, onClose } = props;
  const { findings } = useSurface();
  const [view, setView] = useState<View>("main");
  const [dir, setDir] = useState<"next" | "back">("next");
  useEffect(() => {
    if (open) setView("main");
  }, [open]);
  const go = (v: View) => {
    setDir(v === "main" ? "back" : "next");
    setView(v);
  };
  const noted = findings.filter((f) => state[f.id]?.note);
  const firstLine = context.split("\n")[0];

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
        <h2 className="t-title grow">{titles[view]}</h2>
        <button className="icon-btn" onClick={onClose} aria-label="Close">
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
  return (
    <form
      className="section"
      onSubmit={(e) => {
        e.preventDefault();
        if (changed) onContext(draft.trim());
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
        <Wipe tone="accent" done={!changed && !!context} disabled={!changed}>
          <Icon name="check" />
          {!changed && context ? `Saved for ${agentNames(agentIds)}` : `Save for ${agentNames(agentIds)}`}
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
  return (
    <form
      className="finding-note"
      onSubmit={(e) => {
        e.preventDefault();
        if (changed) onNote(draft.trim().replace(/\.$/, ""));
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
        <Wipe tone={changed ? "accent" : "quiet"} size="s" disabled={!changed}>
          <Icon name="check" />
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
