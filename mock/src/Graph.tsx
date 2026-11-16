import { CSSProperties, MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { categories, raisedShown, Finding, Item, Source } from "./data";
import { useSurface } from "./surface";
import { Count, easing, Icon, token, useReducedMotion, Wipe } from "./ui";
import type { FindingState } from "./App";

type P = { x: number; y: number };
type Hot = { kind: "hub" | "item"; id: string } | null;
type Tip = { x: number; y: number; title: string; sub: string } | null;
type View = { x: number; y: number; k: number };

const RAD = Math.PI / 180;
const HUB_ANGLES = [-112.5, -67.5, -22.5, 22.5, 67.5, 112.5, 157.5, 202.5];
const FLAG_STEP = 34;
const FLAG_ARC = 140;
const MIN_K = 0.35;
const MAX_K = 3.5;
const GRID = 24;
const IDENTITY: View = { x: 0, y: 0, k: 1 };

const shortWhy: Record<string, string> = {
  firewall: "Not collected · needs admin",
  policy: "Not collected · needs admin",
  logs: "Not collected · closed to you",
  files: "Off · you turn it on",
};

const hubKey = (id: string) => `hub:${id}`;

function clamp(n: number, lo: number, hi: number) {
  return Math.min(hi, Math.max(lo, n));
}

function liveStatus(item: Item, state: Record<string, FindingState>) {
  return item.findingId ? state[item.findingId]?.status : undefined;
}

function onGraph(item: Item, state: Record<string, FindingState>) {
  const st = liveStatus(item, state);
  return st === "raised" || st === "asked";
}

type Drawn = P & { o: number };

/**
 * Where each dot is drawn. Targets come from the layout. When a target
 * changes, the dot travels there over --dur-reveal (motion principle 5): a new
 * dot leaves its source node, a dot that is no longer raised returns to it and
 * fades, and a dot already moving starts from where it is (principle 6). A
 * resize or reduced motion places dots at once.
 */
function useDots(targets: Record<string, P>, homes: Record<string, P>, sizeKey: string, delayOf: (id: string) => number): Record<string, Drawn> {
  const reduce = useReducedMotion();
  const [drawn, setDrawn] = useState<Record<string, Drawn>>({});
  const cur = useRef<Record<string, Drawn>>({});
  const home = useRef<Record<string, P>>({});
  const lastSize = useRef(sizeKey);
  const key = Object.keys(targets)
    .sort()
    .map((id) => `${id}:${Math.round(targets[id].x)},${Math.round(targets[id].y)}`)
    .join("|");

  useEffect(() => {
    Object.assign(home.current, homes);
    const to: Record<string, Drawn> = {};
    for (const id in targets) to[id] = { ...targets[id], o: 1 };
    for (const id in cur.current) if (!targets[id] && home.current[id]) to[id] = { ...home.current[id], o: 0 };
    const snap = reduce || lastSize.current !== sizeKey;
    lastSize.current = sizeKey;
    if (snap) {
      const next: Record<string, Drawn> = {};
      for (const id in to) if (to[id].o > 0) next[id] = to[id];
      cur.current = next;
      setDrawn(next);
      return;
    }
    const from: Record<string, Drawn> = {};
    const wait: Record<string, number> = {};
    for (const id in to) {
      from[id] = cur.current[id] ?? { ...(home.current[id] ?? to[id]), o: 0 };
      wait[id] = cur.current[id] ? 0 : delayOf(id);
    }
    const dur = token("--dur-reveal");
    const ease = easing("--ease-out");
    const start = performance.now();
    let raf = 0;
    const step = (now: number) => {
      const next: Record<string, Drawn> = {};
      let moving = false;
      for (const id in to) {
        const t = clamp((now - start - wait[id]) / dur, 0, 1);
        if (t < 1) moving = true;
        else if (to[id].o === 0) continue;
        const e = ease(t);
        const a = from[id];
        const b = to[id];
        next[id] = { x: a.x + (b.x - a.x) * e, y: a.y + (b.y - a.y) * e, o: a.o + (b.o - a.o) * e };
      }
      cur.current = next;
      setDrawn(next);
      if (moving) raf = requestAnimationFrame(step);
    };
    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  }, [key, sizeKey, reduce]); // eslint-disable-line react-hooks/exhaustive-deps

  return drawn;
}

function anchor(dir: P, gap: number): CSSProperties {
  if (Math.abs(dir.x) >= 0.45) return { transform: `translate(${dir.x < 0 ? `calc(-100% - ${gap}px)` : `${gap}px`}, -50%)` };
  return { transform: `translate(-50%, ${dir.y < 0 ? `calc(-100% - ${gap}px)` : `${gap}px`})` };
}

function labelStyle(l: { side: string; q: P; cx: number; y: number }): CSSProperties {
  const gap = token("--label-gap");
  if (l.side === "right") return { left: l.q.x + gap, top: l.y, transform: "translate(0, -50%)" };
  if (l.side === "left") return { left: l.q.x - gap, top: l.y, transform: "translate(-100%, -50%)" };
  return { left: l.cx, top: l.y, transform: `translate(-50%, ${l.side === "up" ? "-100%" : "0"})` };
}

export function Graph({
  entering,
  cascade,
  state,
  onSource,
  onFinding,
  onItem,
}: {
  /** True during the first-load entrance only; see "First load" in components.css. */
  entering: boolean;
  /** True during the reveal after first run: dots and counts arrive source by source. */
  cascade: boolean;
  state: Record<string, FindingState>;
  onSource: (s: Source) => void;
  onFinding: (f: Finding) => void;
  onItem: (s: Source, itemId: string) => void;
}) {
  const { sources, findings, links, host } = useSurface();
  const findingOf = (item: Item) => findings.find((f) => f.id === item.findingId);
  const ref = useRef<HTMLDivElement>(null);
  const viewRef = useRef<View>(IDENTITY);
  const dragRef = useRef<{ px: number; py: number; vx: number; vy: number } | null>(null);
  const movedRef = useRef(false);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [hot, setHot] = useState<Hot>(null);
  const [tip, setTip] = useState<Tip>(null);
  const [lastTip, setLastTip] = useState<Tip>(null);
  const [view, setView] = useState<View>(IDENTITY);
  const [dragging, setDragging] = useState(false);

  const applyView = useCallback((next: View) => {
    viewRef.current = next;
    setView(next);
  }, []);

  useEffect(() => {
    const ro = new ResizeObserver(([e]) => setSize({ w: e.contentRect.width, h: e.contentRect.height }));
    ro.observe(ref.current!);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (tip) setLastTip(tip);
  }, [tip]);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const rect = el.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      const cur = viewRef.current;
      if (e.ctrlKey || e.metaKey) {
        const factor = e.ctrlKey ? Math.exp(-e.deltaY * 0.01) : e.deltaY < 0 ? 1.1 : 1 / 1.1;
        const nextK = clamp(cur.k * factor, MIN_K, MAX_K);
        if (nextK === cur.k) return;
        applyView({
          x: mx - ((mx - cur.x) * nextK) / cur.k,
          y: my - ((my - cur.y) * nextK) / cur.k,
          k: nextK,
        });
        return;
      }
      applyView({ x: cur.x - e.deltaX, y: cur.y - e.deltaY, k: cur.k });
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [applyView]);

  const layout = useMemo(() => {
    const { w, h } = size;
    const center = { x: w * 0.45, y: h * 0.47 };
    const rx = Math.min(w * 0.33, 520);
    const ry = Math.min(h * 0.3, 300);
    const hubR = token("--node-hub") / 2;
    const r0 = hubR + 28;
    const pos: Record<string, P> = {};
    const hubOf: Record<string, string> = {};
    const home: Record<string, P> = {};
    const order: Record<string, { hub: number; k: number }> = {};
    const hubs = sources.map((s, i) => {
      const a = HUB_ANGLES[i] * RAD;
      const dir = { x: Math.cos(a), y: Math.sin(a) };
      const p = { x: center.x + rx * dir.x, y: center.y + ry * dir.y };
      const flagged = s.items.filter((it) => onGraph(it, state));
      const step = Math.min(FLAG_STEP, FLAG_ARC / Math.max(flagged.length - 1, 1));
      flagged.forEach((it, k) => {
        const t = a + (k - (flagged.length - 1) / 2) * step * RAD;
        const r = r0 + Math.floor(k / 8) * 28 + (flagged.length > 4 && k % 2 ? token("--space-5") : 0);
        pos[it.id] = { x: p.x + r * Math.cos(t), y: p.y + r * Math.sin(t) };
        hubOf[it.id] = s.id;
        home[it.id] = p;
        order[it.id] = { hub: i, k };
      });
      return { s, p, dir };
    });
    return { center, hubs, pos, hubOf, home, order, hubR, ring: r0 };
  }, [size, sources, state]);

  const toneOfItem = (it: Item) => {
    const st = liveStatus(it, state);
    return st === "raised" ? "flag" : st === "asked" ? "accent" : undefined;
  };
  const raisedIn = (s: Source) => s.items.filter((it) => toneOfItem(it) === "flag").length;
  const toneOfHub = (s: Source) => (!s.collected ? "empty" : s.items.some((it) => toneOfItem(it) === "flag") ? "flag" : undefined);

  const focus = useMemo(() => {
    if (!hot) return null;
    const set = new Set<string>();
    if (hot.kind === "hub") {
      set.add(hubKey(hot.id));
      sources.find((s) => s.id === hot.id)?.items.forEach((it) => {
        if (layout.pos[it.id]) set.add(it.id);
      });
      return set;
    }
    const item = sources.flatMap((s) => s.items).find((it) => it.id === hot.id);
    if (!item) return set;
    const story = findingOf(item)?.story;
    set.add(item.id);
    links.forEach((l) => {
      if (story ? l.story === story : l.a === item.id || l.b === item.id) {
        set.add(l.a);
        set.add(l.b);
      }
    });
    if (story) findings.filter((f) => f.story === story).forEach((f) => set.add(f.itemId));
    [...set].forEach((id) => layout.hubOf[id] && set.add(hubKey(layout.hubOf[id])));
    return set;
  }, [hot, layout, links, findings, sources]);

  const labelled = useMemo(() => {
    const raised = findings.filter((f) => state[f.id]?.status === "raised").slice(0, raisedShown);
    return new Set([...raised.map((f) => f.id), ...findings.filter((f) => state[f.id]?.status === "asked").map((f) => f.id)]);
  }, [state, findings]);

  const labels = useMemo(() => {
    const gap = token("--label-gap");
    const rowH = token("--control-s") + token("--space-1");
    const edge = token("--space-4");
    type Side = "left" | "right" | "up" | "down";
    const list = layout.hubs.flatMap(({ s, p }) =>
      s.items
        .filter((it) => !!it.findingId && labelled.has(it.findingId) && layout.pos[it.id])
        .flatMap((it) => {
          const f = findingOf(it);
          const q = layout.pos[it.id];
          if (!f || !q || !categories[f.category]) return [];
          const width = (categories[f.category].name.length + f.value.length) * 6.6 + 64;
          const fitsRight = q.x + gap + width < size.w - edge;
          const fitsLeft = q.x - gap - width > edge;
          const prefer: Side = q.x < p.x ? "left" : "right";
          const side: Side = prefer === "right" ? (fitsRight ? "right" : q.y < p.y ? "up" : "down") : fitsLeft ? "left" : q.y < p.y ? "up" : "down";
          const cx = Math.min(Math.max(q.x, edge + width / 2), size.w - edge - width / 2);
          const x0 = side === "right" ? q.x + gap : side === "left" ? q.x - gap - width : cx - width / 2;
          const y = side === "up" ? q.y - gap : side === "down" ? q.y + gap : q.y;
          return [{ it, f, q, side, width, x0, cx, y }];
        }),
    );
    const sorted = list.filter((l) => l.side === "left" || l.side === "right").sort((a, b) => a.y - b.y);
    sorted.forEach((l, i) => {
      l.y = Math.max(l.y, edge + rowH / 2);
      for (let j = 0; j < i; j++) {
        const o = sorted[j];
        const overlapX = l.x0 < o.x0 + o.width && o.x0 < l.x0 + l.width;
        if (overlapX && Math.abs(l.y - o.y) < rowH) l.y = o.y + rowH;
      }
    });
    return list;
  }, [layout, labelled, size.w]);

  const dim = (id: string) => (focus ? !focus.has(id) : false);
  const edgeDim = (a: string, b: string) => (focus ? !(focus.has(a) && focus.has(b)) : false);

  const enterItem = (it: Item) => {
    setHot({ kind: "item", id: it.id });
    const f = findingOf(it);
    const p = layout.pos[it.id];
    if (!p) return;
    if (!f || toneOfItem(it) === undefined) setTip({ x: p.x, y: p.y, title: it.name, sub: it.detail });
    else if (!labelled.has(f.id) && categories[f.category]) setTip({ x: p.x, y: p.y, title: categories[f.category].name, sub: f.value });
  };
  const leave = () => {
    setHot(null);
    setTip(null);
  };
  const clickItem = (s: Source, it: Item) => {
    if (movedRef.current) return;
    const f = findingOf(it);
    if (f && state[f.id]?.status !== "cleared") onFinding(f);
    else onItem(s, it.id);
  };

  const onPointerDown = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    movedRef.current = false;
    const t = e.target as HTMLElement;
    if (t.closest("button, .wipe, a, input, textarea")) return;
    const cur = viewRef.current;
    dragRef.current = { px: e.clientX, py: e.clientY, vx: cur.x, vy: cur.y };
    setDragging(true);
    e.currentTarget.setPointerCapture(e.pointerId);
  };
  const onPointerMove = (e: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    const dx = e.clientX - drag.px;
    const dy = e.clientY - drag.py;
    if (Math.hypot(dx, dy) > 4) movedRef.current = true;
    applyView({ x: drag.vx + dx, y: drag.vy + dy, k: viewRef.current.k });
  };
  const onPointerUp = (e: ReactPointerEvent<HTMLDivElement>) => {
    dragRef.current = null;
    setDragging(false);
    if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId);
    requestAnimationFrame(() => {
      movedRef.current = false;
    });
  };
  const onDoubleClick = (e: ReactMouseEvent<HTMLDivElement>) => {
    const t = e.target as HTMLElement;
    if (t.closest("button, .wipe, a, circle.item")) return;
    applyView(IDENTITY);
  };

  // Arrival order for the build on load and the reveal after first run: one
  // source after another, then each source's dots in turn.
  const hubDelay = (i: number) => {
    if (!entering && !cascade) return 0;
    const stagger = token("--dur-stagger");
    return (entering ? (stagger * 2) / 3 : 0) + (i * stagger) / 5;
  };
  const delayOf = (id: string) => {
    const o = layout.order[id];
    return o ? hubDelay(o.hub) + (o.k * token("--dur-fast")) / 3 : 0;
  };
  const drawn = useDots(layout.pos, layout.home, `${size.w}x${size.h}`, delayOf);
  const at = (id: string) => drawn[id];
  // A label whose dot is leaving stays on it until the dot is gone.
  const lastLabels = useRef(new Map<string, (typeof labels)[number]>());
  for (const l of labels) lastLabels.current.set(l.it.id, l);
  const current = new Set(labels.map((l) => l.it.id));
  const shownLabels = [...labels, ...Object.keys(drawn).filter((id) => !current.has(id) && !layout.pos[id] && lastLabels.current.has(id)).map((id) => lastLabels.current.get(id)!)];

  const ready = size.w > 0;
  const { center, hubs, hubR } = layout;
  // A dot that is leaving may belong to an item the latest store no longer has,
  // so the last item seen for each id is kept for its exit.
  const seenItems = useRef(new Map<string, { it: Item; s: Source }>());
  for (const s of sources) for (const it of s.items) seenItems.current.set(it.id, { it, s });
  const itemsById = seenItems.current;
  const shownTip = tip ?? lastTip;

  return (
    <div
      className="graph"
      ref={ref}
      data-drag={dragging}
      data-entering={entering}
      style={{
        backgroundSize: `${GRID * view.k}px ${GRID * view.k}px`,
        backgroundPosition: `${view.x}px ${view.y}px`,
      }}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onDoubleClick={onDoubleClick}
    >
      {ready && (
        <div className="graph__world" style={{ transform: `translate(${view.x}px, ${view.y}px) scale(${view.k})` }}>
          <svg className="graph__svg" width={size.w} height={size.h} aria-hidden>
            {hubs.map(({ s, p }) => (
              <line key={`spine-${s.id}`} className="edge edge--spine" x1={center.x} y1={center.y} x2={p.x} y2={p.y} data-tone={s.collected ? undefined : "empty"} data-hot={hot?.kind === "hub" && hot.id === s.id} data-dim={dim(hubKey(s.id))} />
            ))}
            {Object.entries(drawn).map(([id, d]) => {
              const hit = itemsById.get(id);
              const h = layout.home[id];
              if (!h) return null;
              return (
                <g key={`e-${id}`} opacity={d.o}>
                  <line className="edge" x1={h.x} y1={h.y} x2={d.x} y2={d.y} data-tone={hit ? toneOfItem(hit.it) : undefined} data-dim={hit ? edgeDim(hubKey(hit.s.id), id) : false} />
                </g>
              );
            })}
            {links.map((l) => {
              const a = at(l.a);
              const b = at(l.b);
              if (!a || !b) return null;
              const mid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
              const c = { x: mid.x + (center.x - mid.x) * 0.45, y: mid.y + (center.y - mid.y) * 0.45 };
              const raised = [l.a, l.b].some((id) => {
                const it = sources.flatMap((s) => s.items).find((x) => x.id === id);
                return it ? toneOfItem(it) === "flag" : false;
              });
              return <path key={`${l.a}-${l.b}`} opacity={Math.min(a.o, b.o)} className="edge edge--link" d={`M${a.x} ${a.y} Q${c.x} ${c.y} ${b.x} ${b.y}`} data-tone={l.story && raised ? "flag" : undefined} data-dim={edgeDim(l.a, l.b)} />;
            })}
            {hubs.filter(({ s }) => !s.collected).map(({ s, p }) => (
              <circle key={`ring-${s.id}`} className="ring" cx={p.x} cy={p.y} r={layout.ring} data-dim={dim(hubKey(s.id))} />
            ))}
            {Object.entries(drawn).map(([id, d]) => {
              const hit = itemsById.get(id);
              if (!hit) return null;
              const { it, s } = hit;
              const tone = toneOfItem(it);
              const r = tone ? token("--node-flag") : token("--node-item");
              return (
                <g key={`n-${it.id}`} opacity={d.o}>
                  <circle
                    className="item"
                    cx={d.x}
                    cy={d.y}
                    r={r}
                    data-tone={tone}
                    data-hot={hot?.id === it.id}
                    data-dim={dim(it.id)}
                    onMouseEnter={() => enterItem(it)}
                    onMouseLeave={leave}
                    onClick={() => clickItem(s, it)}
                  />
                </g>
              );
            })}
          </svg>

          <div className="node node--root" style={{ left: center.x, top: center.y }}>
            <Icon name="laptop" />
          </div>
          <div className="label" style={{ left: center.x, top: center.y + token("--node-root") / 2 + 6, transform: "translateX(-50%)", textAlign: "center" }}>
            <span className="label__name">{host.name}</span>
            <span className="label__sub">
              {host.os} · as {host.user}
            </span>
          </div>

          {hubs.map(({ s, p, dir }, i) => {
            const inward = { x: -dir.x, y: -dir.y };
            const at = { left: p.x + inward.x * (hubR + 2), top: p.y + inward.y * (hubR + 2) };
            return (
              <div key={`hub-${s.id}`} style={{ display: "contents" }}>
                <button
                  className="node node--hub"
                  style={{ left: p.x, top: p.y }}
                  data-tone={toneOfHub(s)}
                  data-hot={hot?.kind === "hub" && hot.id === s.id}
                  data-dim={dim(hubKey(s.id))}
                  aria-label={s.name}
                  onMouseEnter={() => {
                    setHot({ kind: "hub", id: s.id });
                    setTip({ x: p.x, y: p.y - hubR, title: s.name, sub: s.looksAt });
                  }}
                  onMouseLeave={leave}
                  onClick={() => {
                    if (movedRef.current) return;
                    onSource(s);
                  }}
                >
                  <Icon name={s.icon} />
                  {raisedIn(s) > 0 && (
                    <span className="hub-badge" style={{ animationDelay: `${hubDelay(i)}ms` }}>
                      <Count value={raisedIn(s)} delay={hubDelay(i)} />
                    </span>
                  )}
                </button>
                <div className="label" style={{ ...at, ...anchor(inward, 8) }} data-dim={dim(hubKey(s.id))}>
                  <span className="label__name">
                    {s.name} {s.collected && <Count className="muted num" value={s.items.length} delay={hubDelay(i)} />}
                  </span>
                  <span className="label__sub">{s.collected ? s.tileFeed : shortWhy[s.id]}</span>
                </div>
              </div>
            );
          })}

          <svg className="graph__svg graph__svg--over" width={size.w} height={size.h} aria-hidden>
            {shownLabels.map((l) => {
              const d = at(l.it.id);
              if (!d) return null;
              const x2 = l.side === "right" ? l.q.x + token("--label-gap") : l.side === "left" ? l.q.x - token("--label-gap") : l.q.x;
              return (
                <g key={`ld-${l.it.id}`} opacity={d.o}>
                  <line className="leader" data-tone={toneOfItem(l.it)} data-dim={dim(l.it.id)} x1={d.x} y1={d.y} x2={d.x + x2 - l.q.x} y2={d.y + l.y - l.q.y} />
                </g>
              );
            })}
          </svg>
          {shownLabels.map((l) => {
            const d = at(l.it.id);
            if (!d) return null;
            // A label rides with its dot: same offset from the dot, same opacity.
            const moved = { ...l, q: { x: d.x, y: d.y }, cx: l.cx + d.x - l.q.x, y: l.y + d.y - l.q.y };
            return (
            <div key={`fl-${l.it.id}`} style={{ opacity: d.o }}>
            <div className="flag-label" data-tone={toneOfItem(l.it)} data-dim={dim(l.it.id)} style={labelStyle(moved)}>
              <Wipe tone="flag" size="s" onClick={() => onFinding(l.f)} onMouseEnter={() => setHot({ kind: "item", id: l.it.id })} onMouseLeave={leave}>
                <span className="cat-tile cat-tile--s" data-cat={categories[l.f.category].color}>
                  <Icon name={toneOfItem(l.it) === "accent" ? "agent" : categories[l.f.category].icon} />
                </span>
                {categories[l.f.category].name}
                <span className="flag-label__rest">{l.f.value}</span>
              </Wipe>
            </div>
            </div>
            );
          })}

          <div className="tip" data-open={!!tip} style={{ left: shownTip?.x, top: shownTip?.y }}>
            <span className="t-strong">{shownTip?.title}</span>
            <span>{shownTip?.sub}</span>
          </div>
        </div>
      )}

      <div className="graph__foot">
        <p className="graph__honest">
          A snapshot of what a normal user can list. Short-lived programs are invisible. Firewall, device policy, and kernel telemetry are not on this picture, because this agent cannot see them. This page does not know if a file is malicious.
        </p>
        <div className="legend">
          <span>
            <span className="dot" /> Raised
          </span>
          <span>
            <span className="dot dot--accent" /> With your agent
          </span>
          <span>
            <span className="dot dot--quiet" /> On the surface
          </span>
          <span>
            <span className="ring-dot" /> Not collected
          </span>
        </div>
      </div>
    </div>
  );
}
