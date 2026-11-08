import { ReactNode, TransitionEvent, isValidElement, useEffect, useRef, useState } from "react";
import type { IconName } from "./data";

function useReducedMotion() {
  const [reduce, setReduce] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduce(mq.matches);
    const onChange = () => setReduce(mq.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);
  return reduce;
}

/** Stable key for Swap: prefer an explicit swapKey, else derive from text/icons. */
function contentKey(node: ReactNode): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(contentKey).join("|");
  if (isValidElement(node)) {
    const props = node.props as { children?: ReactNode; name?: string };
    return `${String(node.key ?? "")}:${props.name ?? ""}:${contentKey(props.children)}`;
  }
  return "";
}

export function token(name: string): number {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return parseFloat(raw) || 0;
}

const paths: Record<IconName | "laptop" | "chevron" | "check" | "copy" | "close" | "agent" | "pause" | "play" | "arrow" | "undo" | "lock" | "settings" | "back", ReactNode> = {
  laptop: (
    <>
      <rect x="4" y="5" width="16" height="11" />
      <path d="M2 19h20" />
    </>
  ),
  process: (
    <>
      <rect x="6" y="6" width="12" height="12" />
      <rect x="10" y="10" width="4" height="4" />
      <path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3" />
    </>
  ),
  network: (
    <>
      <circle cx="6" cy="12" r="2.5" />
      <circle cx="18" cy="6" r="2.5" />
      <circle cx="18" cy="18" r="2.5" />
      <path d="M8.3 10.9l7.4-3.8M8.3 13.1l7.4 3.8" />
    </>
  ),
  file: (
    <>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5" />
    </>
  ),
  apps: (
    <>
      <rect x="4" y="4" width="7" height="7" />
      <rect x="13" y="4" width="7" height="7" />
      <rect x="4" y="13" width="7" height="7" />
      <rect x="13" y="13" width="7" height="7" />
    </>
  ),
  startup: (
    <>
      <path d="M12 3v8" />
      <path d="M6.3 7.2a8 8 0 1 0 11.4 0" />
    </>
  ),
  shield: <path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z" />,
  policy: (
    <>
      <rect x="5" y="4" width="14" height="17" />
      <path d="M9 4V3h6v1M9 12l2 2 4-4" />
    </>
  ),
  logs: <path d="M5 6h14M5 10h14M5 14h9M5 18h6" />,
  globe: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z" />
    </>
  ),
  signal: (
    <>
      <path d="M7 17L17 7M9 7h8v8" />
      <path d="M4 20h4" />
    </>
  ),
  eyeOff: (
    <>
      <path d="M3 3l18 18" />
      <path d="M10.6 5.1A9.6 9.6 0 0 1 12 5c5 0 9 5 9 7a10 10 0 0 1-2.6 3.4M6.6 6.6C4.4 8 3 10.4 3 12c0 2 4 7 9 7a9 9 0 0 0 4.4-1.1" />
      <path d="M9.9 9.9a3 3 0 0 0 4.2 4.2" />
    </>
  ),
  fileWarning: (
    <>
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5M12 11v3.5M12 17.5v.01" />
    </>
  ),
  login: (
    <>
      <path d="M14 4h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4" />
      <path d="M4 12h11M11 8l4 4-4 4" />
    </>
  ),
  badgeX: (
    <>
      <path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z" />
      <path d="M9.5 9.5l5 5M14.5 9.5l-5 5" />
    </>
  ),
  chevron: <path d="M6 9l6 6 6-6" />,
  check: <path d="M5 12.5l4.5 4.5L19 7.5" />,
  copy: (
    <>
      <rect x="8" y="8" width="12" height="12" />
      <path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" />
    </>
  ),
  close: <path d="M6 6l12 12M18 6L6 18" />,
  agent: <path d="M5 5h14a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-7l-5 4v-4H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2z" />,
  pause: <path d="M9 6v12M15 6v12" />,
  play: <path d="M8 5l11 7-11 7z" />,
  arrow: <path d="M5 12h14M13 6l6 6-6 6" />,
  back: <path d="M19 12H5M11 6l-6 6 6 6" />,
  undo: (
    <>
      <path d="M9 14L4 9l5-5" />
      <path d="M4 9h10a6 6 0 0 1 0 12h-3" />
    </>
  ),
  settings: (
    <>
      <path d="M4 7h10M18 7h2M4 17h4M12 17h8" />
      <circle cx="16" cy="7" r="2" />
      <circle cx="10" cy="17" r="2" />
    </>
  ),
  lock: (
    <>
      <rect x="5" y="11" width="14" height="10" />
      <path d="M8 11V8a4 4 0 0 1 8 0v3" />
    </>
  ),
};

export type AnyIcon = keyof typeof paths;

export function Icon({ name }: { name: AnyIcon }) {
  return (
    <svg className="icon" viewBox="0 0 24 24" aria-hidden>
      {paths[name]}
    </svg>
  );
}

/**
 * In-place content wipe. Same left-to-right clip as Wipe, for titles, details,
 * and any label that changes without unmounting the parent.
 * Pass swapKey when children update often (tickers) but should only wipe on real state changes.
 */
export function Swap({
  children,
  swapKey,
  block,
  className = "",
}: {
  children: ReactNode;
  swapKey?: string | number | boolean;
  block?: boolean;
  className?: string;
}) {
  const key = swapKey !== undefined ? String(swapKey) : contentKey(children);
  const reduce = useReducedMotion();
  const [current, setCurrent] = useState(children);
  const [currentKey, setCurrentKey] = useState(key);
  const [incoming, setIncoming] = useState<ReactNode>(null);
  const [incomingKey, setIncomingKey] = useState(key);
  const [phase, setPhase] = useState<"idle" | "prep" | "go">("idle");

  useEffect(() => {
    if (key === currentKey && phase === "idle") {
      setCurrent(children);
    }
  }, [children, key, currentKey, phase]);

  useEffect(() => {
    if (key === currentKey) return;
    if (reduce) {
      setCurrent(children);
      setCurrentKey(key);
      setIncoming(null);
      setPhase("idle");
      return;
    }
    setIncoming(children);
    setIncomingKey(key);
    setPhase("prep");
  }, [key, children, currentKey, reduce]);

  useEffect(() => {
    if (phase !== "prep") return;
    const id = requestAnimationFrame(() => {
      requestAnimationFrame(() => setPhase("go"));
    });
    return () => cancelAnimationFrame(id);
  }, [phase]);

  const finish = (e: TransitionEvent<HTMLSpanElement>) => {
    if (phase !== "go" || e.propertyName !== "clip-path") return;
    setCurrent(incoming);
    setCurrentKey(incomingKey);
    setIncoming(null);
    setPhase("idle");
  };

  return (
    <span className={["swap", block ? "swap--block" : "", className].filter(Boolean).join(" ")} data-phase={phase === "go" ? "go" : phase === "prep" ? "prep" : "idle"}>
      <span className="swap__base">{current}</span>
      {incoming != null && (
        <span className="swap__layer" onTransitionEnd={finish}>
          {incoming}
        </span>
      )}
    </span>
  );
}

type WipeProps = {
  tone?: "accent" | "flag" | "quiet";
  size?: "s" | "m";
  block?: boolean;
  done?: boolean;
  disabled?: boolean;
  className?: string;
  onClick?: () => void;
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
  children: ReactNode;
  label?: string;
  /** When set, label Swap only fires when this changes (plus done). */
  swapKey?: string | number | boolean;
};

export function Wipe({ tone = "accent", size = "m", block, done, disabled, className = "", onClick, onMouseEnter, onMouseLeave, children, label, swapKey }: WipeProps) {
  const cls = ["wipe", `tone-${tone}`, size === "s" ? "wipe--s" : "", block ? "wipe--block" : "", className].join(" ");
  const key = swapKey !== undefined ? `${swapKey}:${done ? "1" : "0"}` : undefined;
  const face = (
    <Swap className="wipe__face" swapKey={key}>
      {children}
    </Swap>
  );
  return (
    <button className={cls} data-done={done ? "true" : undefined} disabled={disabled} onClick={onClick} onMouseEnter={onMouseEnter} onMouseLeave={onMouseLeave} aria-label={label}>
      {face}
      <span className="wipe__layer" aria-hidden>
        <Swap className="wipe__face" swapKey={key}>
          {children}
        </Swap>
      </span>
    </button>
  );
}

export function Chevron({ open }: { open: boolean }) {
  return (
    <span className="chevron" data-open={open}>
      <Icon name="chevron" />
    </span>
  );
}

export function Reveal({ open, children }: { open: boolean; children: ReactNode }) {
  return (
    <div className="reveal" data-open={open}>
      <div className="reveal__inner">{children}</div>
    </div>
  );
}

export function useOutside(ref: React.RefObject<HTMLElement>, onOut: () => void, active: boolean) {
  useEffect(() => {
    if (!active) return;
    const down = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onOut();
    };
    const key = (e: KeyboardEvent) => e.key === "Escape" && onOut();
    document.addEventListener("mousedown", down);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("mousedown", down);
      document.removeEventListener("keydown", key);
    };
  }, [active, onOut, ref]);
}

export function Menu<T extends string>({ value, options, onChange }: { value: T; options: { value: T; label: string }[]; onChange: (v: T) => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useOutside(ref, () => setOpen(false), open);
  const current = options.find((o) => o.value === value);
  return (
    <div className="menu" ref={ref}>
      <button className="menu__trigger" data-open={open} onClick={() => setOpen(!open)} aria-expanded={open}>
        {current?.label}
        <Chevron open={open} />
      </button>
      <div className="menu__pop" data-open={open}>
        <Reveal open={open}>
          <ul className="menu__list" role="listbox">
            {options.map((o) => (
              <li key={o.value}>
                <button
                  className="menu__item"
                  data-selected={o.value === value}
                  onClick={() => {
                    onChange(o.value);
                    setOpen(false);
                  }}
                >
                  {o.label}
                  {o.value === value && <Icon name="check" />}
                </button>
              </li>
            ))}
          </ul>
        </Reveal>
      </div>
    </div>
  );
}

export function Row({ lead, title, detail, meta, tone, children, open, onToggle }: { lead?: ReactNode; title: ReactNode; detail?: ReactNode; meta?: ReactNode; tone?: "flag" | "accent" | "quiet"; children: ReactNode; open: boolean; onToggle: () => void }) {
  return (
    <div className="row" data-open={open} data-tone={tone}>
      <button className="row__head" onClick={onToggle} aria-expanded={open}>
        {lead}
        <span className="row__text">
          <span className="row__title">{title}</span>
          {detail && <span className="row__detail">{detail}</span>}
        </span>
        {meta && <span className="row__meta">{meta}</span>}
        <Chevron open={open} />
      </button>
      <Reveal open={open}>
        <div className="row__body">{children}</div>
      </Reveal>
    </div>
  );
}

export function Scrim({ open, onClose, level = 1 }: { open: boolean; onClose: () => void; level?: number }) {
  return <div className="scrim" data-open={open} data-level={level} onClick={onClose} />;
}

export function useLatched<T>(value: T | null): T | null {
  const last = useRef<T | null>(value);
  if (value !== null) last.current = value;
  return value ?? last.current;
}

export function useEscape(onEsc: () => void, active: boolean) {
  useEffect(() => {
    if (!active) return;
    const key = (e: KeyboardEvent) => e.key === "Escape" && onEsc();
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [active, onEsc]);
}

export function CopyButton({ text, tone = "accent", size = "m", label = "Copy", onCopied }: { text: string; tone?: WipeProps["tone"]; size?: WipeProps["size"]; label?: string; onCopied?: () => void }) {
  const [copied, setCopied] = useState(false);
  const click = () => {
    navigator.clipboard?.writeText(text).catch(() => {});
    setCopied(true);
    onCopied?.();
    window.setTimeout(() => setCopied(false), token("--dur-hold"));
  };
  return (
    <Wipe tone={tone} size={size} done={copied} swapKey={copied ? "copied" : "idle"} onClick={click}>
      <Icon name={copied ? "check" : "copy"} />
      {copied ? "Copied" : label}
    </Wipe>
  );
}

export function CopyField({ label, text, tone = "accent", onCopied }: { label: string; text: string; tone?: WipeProps["tone"]; onCopied?: () => void }) {
  return (
    <label className="field">
      <span className="field__label">{label}</span>
      <span className="field__row">
        <input className="field__input" readOnly value={text} onFocus={(e) => e.currentTarget.select()} />
        <CopyButton text={text} tone={tone} onCopied={onCopied} />
      </span>
    </label>
  );
}

export function Toast({ message, action, onAction, open }: { message: ReactNode; action?: string; onAction?: () => void; open: boolean }) {
  return (
    <div className="toast on-dark" data-open={open} role="status">
      <Icon name="check" />
      <span>{message}</span>
      {action && (
        <Wipe tone="quiet" size="s" onClick={onAction}>
          <Icon name="undo" />
          {action}
        </Wipe>
      )}
    </div>
  );
}

export function Toggle({ on, disabled, onChange, label }: { on: boolean; disabled?: boolean; onChange?: (v: boolean) => void; label: string }) {
  return (
    <button className="toggle" role="switch" aria-checked={on} aria-label={label} data-on={on} disabled={disabled} onClick={() => onChange?.(!on)}>
      <span className="toggle__knob" />
    </button>
  );
}
