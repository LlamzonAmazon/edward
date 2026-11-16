import { useState } from "react";
import { askSketchy, categories, raisedShown, Finding, Source } from "./data";
import { useSurface } from "./surface";
import { Chevron, CopyButton, Count, Icon, Reveal, Swap, token } from "./ui";
import type { FindingState, Status } from "./App";
import logo from "./assets/edward-logo.png";

type Props = {
  state: Record<string, FindingState>;
  activeFinding: string | null;
  activeSource: string | null;
  onFinding: (f: Finding) => void;
  onSource: (s: Source) => void;
  paused: boolean;
  tick: boolean;
  readAgo: string;
  onPause: () => void;
  onAll: () => void;
  onSettings: () => void;
  settingsOpen: boolean;
  /** True during the reveal after first run: rows arrive one after another. */
  cascade: boolean;
};

export function Nav(props: Props) {
  const { state, activeFinding, activeSource, onFinding, onSource, paused, tick, readAgo, onPause, onAll, onSettings, settingsOpen, cascade } = props;
  const { sources, findings, host } = useSurface();
  const [showCleared, setShowCleared] = useState(false);
  const by = (status: Status) => findings.filter((f) => (state[f.id]?.status ?? "raised") === status);
  const raised = by("raised");
  const asked = by("asked");
  const cleared = by("cleared");
  const shownRaised = new Set(raised.slice(0, raisedShown).map((f) => f.id));
  // In the reveal the sidebar starts halfway through the graph's build, then
  // each row follows the one above it.
  const start = cascade ? token("--dur-stagger") / 2 : 0;
  const after = (i: number) => (cascade ? start + (i * token("--dur-fast")) / 2 : 0);
  const order = new Map(raised.slice(0, raisedShown).map((f, i) => [f.id, i]));

  const item = (f: Finding, open: boolean) => {
    const cat = categories[f.category];
    if (!cat) return null;
    const st = state[f.id]?.status ?? "raised";
    return (
      <Reveal key={f.id} open={open} appear delay={after((order.get(f.id) ?? 0) + 1)}>
        <button className="nav__item" data-active={activeFinding === f.id} onClick={() => onFinding(f)} tabIndex={open ? 0 : -1}>
          <span className="cat-tile" data-cat={cat.color}>
            <Icon name={cat.icon} />
          </span>
          <span className="nav__text">
            <span className="nav__title">{cat.name}</span>
            <span className="nav__sub">{f.value}</span>
          </span>
          {st === "raised" ? <span className="dot" /> : st === "cleared" ? <span className="dot dot--quiet" /> : <span className="dot dot--accent" />}
        </button>
      </Reveal>
    );
  };

  return (
    <nav className="nav on-dark">
      <div className="nav__brand">
        <img className="wordmark" src={logo} alt="EDwaRd" />
        <span className="nav__host">{host.name}</span>
      </div>
      <div className="nav__scroll">
        <div className="nav__group">
          <div className="nav__heading">
            Raised <Count className="nav__count num" value={raised.length} delay={after(0)} />
          </div>
          {findings.map((f) => item(f, shownRaised.has(f.id)))}
          <Reveal open={!raised.length}>
            <p className="nav__empty">Nothing raised right now.</p>
          </Reveal>
          <Reveal open={raised.length > raisedShown}>
            <button className="nav__item nav__more" onClick={onAll}>
              <span className="nav__text">
                <span className="nav__title">See all {raised.length} raised</span>
              </span>
              <Icon name="arrow" />
            </button>
          </Reveal>
        </div>

        <Reveal open={asked.length > 0}>
          <div className="nav__group">
            <div className="nav__heading">
              With your agent <span className="nav__count">{asked.length}</span>
            </div>
            {findings.map((f) => item(f, state[f.id].status === "asked"))}
          </div>
        </Reveal>

        <Reveal open={cleared.length > 0}>
          <div className="nav__group">
            <button className="nav__heading" onClick={() => setShowCleared(!showCleared)} aria-expanded={showCleared}>
              Cleared <Chevron open={showCleared} />
              <span className="nav__count">{cleared.length}</span>
            </button>
            {findings.map((f) => item(f, showCleared && state[f.id].status === "cleared"))}
          </div>
        </Reveal>

        <div className="nav__group">
          <div className="nav__heading">
            Sources <span className="nav__count">{sources.filter((s) => s.collected).length} of {sources.length}</span>
          </div>
          {sources.map((s, i) => (
            <button key={s.id} className="nav__item" data-active={activeSource === s.id} data-off={!s.collected} onClick={() => onSource(s)}>
              <Icon name={s.icon} />
              <span className="nav__text">
                <span className="nav__title">{s.name}</span>
              </span>
              <span className="nav__meta">{s.collected ? <Count value={s.items.length} delay={after(raisedShown + 1 + i)} /> : s.optIn ? "Off" : <Icon name="lock" />}</span>
            </button>
          ))}
        </div>
      </div>
      <div className="nav__foot">
        <CopyButton text={askSketchy} label="Ask your agent" />
        <div className="nav__foot-row">
          <button className="status" data-paused={paused} onClick={onPause} aria-label={paused ? "Resume" : "Pause"}>
            <span className="dot" data-tick={tick} key={paused ? "p" : tick ? "tick" : "idle"} />
            <Swap className="status__label" swapKey={paused ? "paused" : "looking"}>
              {paused ? "Paused" : `Looking · ${readAgo.toLowerCase()}`}
            </Swap>
            <Swap className="status__icon" swapKey={paused ? "play" : "pause"}>
              <Icon name={paused ? "play" : "pause"} />
            </Swap>
          </button>
          <button className="icon-btn" data-active={settingsOpen} onClick={onSettings} aria-label="Settings">
            <Icon name="settings" />
          </button>
        </div>
      </div>
    </nav>
  );
}
