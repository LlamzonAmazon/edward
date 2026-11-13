import { useState } from "react";
import { categories, Finding, notePicks } from "./data";
import { CopyButton, Icon, Swap, Wipe } from "./ui";
import type { FindingState } from "./App";

type Props = {
  finding: Finding;
  st: FindingState;
  onClose: () => void;
  agentNames: string;
  onClear: () => void;
  onAsk: () => void;
  onNote: (note: string | undefined) => void;
};

export function FindingView(props: Props) {
  const { finding, st, onClose } = props;
  const cat = categories[finding.category];
  return (
    <>
      <div className="dialog__head">
        <span className="cat-tile cat-tile--l" data-cat={cat.color}>
          <Icon name={cat.icon} />
        </span>
        <h2 className="t-title finding__title grow">
          {cat.name} <b>{finding.value}</b>
        </h2>
        <button className="icon-btn" onClick={onClose} aria-label="Close">
          <Icon name="close" />
        </button>
      </div>
      <Detail {...props} />
    </>
  );
}

function isPathLike(label: string, value: string) {
  return label === "Command" || value.startsWith("~") || value.startsWith("/");
}

function Chips({ finding, st }: { finding: Finding; st: FindingState }) {
  const chip =
    st.status === "asked" ? (
      <span className="chip chip--accent">
        <Icon name="agent" />
        With your agent
      </span>
    ) : st.status === "cleared" ? (
      <span className="chip">Cleared by you</span>
    ) : null;
  return (
    <div className="chips">
      {chip && (
        <Swap swapKey={st.status} className="chips__swap">
          {chip}
        </Swap>
      )}
      {finding.facts.map((f) => (
        <span key={f} className="chip">
          {f}
        </span>
      ))}
    </div>
  );
}

function Detail({ finding, st, agentNames, onClear, onAsk, onNote }: Props) {
  const [draft, setDraft] = useState("");
  const cat = categories[finding.category];
  const ask = [
    `What is this? EDwaRd raised “${cat.name}”: ${finding.value}.`,
    ...finding.quoted.map(([k, v]) => `${k}: ${v}.`),
    `${finding.facts.join(", ")}.`,
    `Started by ${finding.chain.join(" → ")}.`,
    st.note ? `I told EDwaRd: “${st.note}.”` : "",
    "Read the rest from the EDwaRd store. Don’t scan the machine.",
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <>
      <div className="dialog__body">
        <Chips finding={finding} st={st} />
        <div className="finding__grid">
          <div className="stack stack--loose">
            <div className="readings">
              <div className="reading reading--real">
                <span className="reading__head">
                  <span className="dot" />
                  Might be real
                </span>
                <p>{cat.real}</p>
              </div>
              <div className="reading reading--fine">
                <span className="reading__head">
                  <span className="dot" />
                  Might be fine
                </span>
                <p>{cat.fine}</p>
              </div>
            </div>

            <div className="step" key={st.note ? "saved" : "edit"} data-dir="next">
              {st.note ? (
                <div className="field">
                  <span className="field__label">Saved for {agentNames}</span>
                  <div className="note">
                    <Icon name="agent" />
                    <span>“{st.note}.”</span>
                    <button className="note__edit" onClick={() => onNote(undefined)}>
                      Change
                    </button>
                  </div>
                </div>
              ) : (
                <form
                  className="field"
                  onSubmit={(e) => {
                    e.preventDefault();
                    if (draft.trim()) onNote(draft.trim().replace(/\.$/, ""));
                  }}
                >
                  <label className="field__label" htmlFor="note">
                    Your note
                  </label>
                  <div className="picks">
                    {notePicks.map((p) => (
                      <Wipe key={p} tone="quiet" size="s" onClick={() => onNote(p)}>
                        {p}
                      </Wipe>
                    ))}
                  </div>
                  <input id="note" className="field__input" value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="Or write your own" />
                  <div>
                    <Wipe tone={draft.trim() ? "accent" : "quiet"} size="s" disabled={!draft.trim()}>
                      <Icon name="arrow" />
                      Save note for {agentNames}
                    </Wipe>
                  </div>
                </form>
              )}
            </div>
          </div>

          <div className="stack stack--loose">
            <div className="field">
              <span className="field__label">Started by</span>
              <ol className="chain">
                {finding.chain.map((c) => (
                  <li key={c}>
                    <span className="dot" />
                    {c}
                  </li>
                ))}
              </ol>
            </div>
            <dl className="details">
              {finding.quoted.map(([k, v]) =>
                isPathLike(k, v) ? (
                  <div key={k} className="copy-row">
                    <dt>{k}</dt>
                    <dd>
                      <CopyButton text={v} tone="quiet" size="s" label={`Copy ${k.toLowerCase()}`} />
                    </dd>
                  </div>
                ) : (
                  <div key={k}>
                    <dt>{k}</dt>
                    <dd>{v}</dd>
                  </div>
                ),
              )}
            </dl>
          </div>
        </div>
      </div>
      <div className="dialog__foot">
        <span className="spacer" />
        {st.status !== "cleared" && (
          <>
            <Wipe tone="quiet" onClick={onClear}>
              <Icon name="check" />
              Clear
            </Wipe>
            <CopyButton text={ask} label="Ask your agent what this is" onCopied={onAsk} />
          </>
        )}
      </div>
    </>
  );
}
