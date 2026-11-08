import { useEffect, useMemo, useState } from "react";
import { categories, Finding, Item, Source } from "./data";
import { useSurface } from "./surface";
import { CopyButton, Icon, Menu, Row, Toggle, Wipe } from "./ui";
import type { FindingState } from "./App";

type Order = "raised" | "az" | "only";

const orders: { value: Order; label: string }[] = [
  { value: "raised", label: "Raised first" },
  { value: "az", label: "A to Z" },
  { value: "only", label: "Only raised" },
];

const nouns: Record<string, string> = {
  processes: "running programs",
  connections: "connections",
  files: "new files",
  apps: "apps and packages",
  startup: "things that start at login",
};

const singular: Record<string, string> = {
  processes: "a running program",
  connections: "a connection",
  files: "a new file",
  apps: "an app",
  startup: "something that starts at login",
};

function askAbout(source: string, item: Item, f?: Finding) {
  return [
    `What is this? EDwaRd shows ${singular[source]}: ${item.name}.`,
    f ? `EDwaRd raised it as “${categories[f.category].name}”.` : "",
    ...item.facts.map(([k, v]) => `${k}: ${v}.`),
    "Read the rest from the EDwaRd store. Don’t scan the machine.",
  ]
    .filter(Boolean)
    .join(" ");
}

const fact = (item: Item, key: string) => item.facts.find(([k]) => k === key)?.[1] ?? "";
const base = (path: string) => path.split("/").filter(Boolean).pop() ?? path;

function rowDetail(source: string, item: Item) {
  if (source === "processes") return `Started by ${fact(item, "Started by")}`;
  if (source === "files") return fact(item, "Kind");
  if (source === "startup") return `Runs ${base(item.detail)}`;
  return item.detail;
}

export function Shelf({ source, state, readAgo, initialRow, onFinding, onClose }: { source: Source; state: Record<string, FindingState>; readAgo: string; initialRow: string | null; onFinding: (f: Finding) => void; onClose: () => void }) {
  const { findings, setFilesOn } = useSurface();
  const [order, setOrder] = useState<Order>("raised");
  const [openRow, setOpenRow] = useState<string | null>(null);

  const findingOf = (item: Item) => findings.find((f) => f.id === item.findingId);
  const statusOf = (item: Item) => (item.findingId ? state[item.findingId]?.status : null);
  const raisedCount = source.items.filter((i) => statusOf(i) === "raised").length;

  useEffect(() => {
    setOrder("raised");
    const first = source.items.find((i) => statusOf(i) === "raised");
    setOpenRow(initialRow ?? (first ? first.id : null));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source.id, initialRow]);

  const items = useMemo(() => {
    const rank = (i: Item) => (statusOf(i) === "raised" ? 0 : statusOf(i) === "asked" ? 1 : 2);
    const list = [...source.items];
    if (order === "az") return list.sort((a, b) => a.name.localeCompare(b.name));
    if (order === "only") return list.filter((i) => statusOf(i) === "raised" || statusOf(i) === "asked");
    return list.sort((a, b) => rank(a) - rank(b));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [order, source, state]);

  const cells = source.collected ? source.items.map((i) => ({ id: i.id, tone: statusOf(i) === "raised" ? "flag" : statusOf(i) === "asked" ? "accent" : undefined })) : Array.from({ length: 24 }, (_, n) => ({ id: String(n), tone: undefined }));

  return (
    <>
      <div className="sheet__head">
        <span className="icon-box" data-tone={raisedCount ? "flag" : undefined}>
          <Icon name={source.icon} />
        </span>
        <div className="grow">
          <h2 className="t-title">{source.name}</h2>
          <span className="muted t-strong">
            {source.collected ? (raisedCount ? `${source.items.length} on the shelf · ${raisedCount} raised` : `${source.items.length} on the shelf · nothing raised`) : "Empty shelf"}
          </span>
        </div>
        <button className="icon-btn" onClick={onClose} aria-label="Close">
          <Icon name="close" />
        </button>
      </div>
      <div className="sheet__body">
        <div className="stock" data-empty={!source.collected} aria-hidden>
          {cells.map((c) => (
            <span key={c.id} className="stock__cell" data-tone={c.tone} />
          ))}
        </div>

        <dl className="feed">
          <dt>Looks at</dt>
          <dd>{source.looksAt}</dd>
          <dt>Feed</dt>
          <dd>{source.feed}</dd>
          {source.collected && (
            <>
              <dt>Read</dt>
              <dd className="num">{readAgo}</dd>
            </>
          )}
        </dl>

        {source.collected ? (
          <>
            <div className="list-head">
              <span className="t-head">
                {source.items.length} {nouns[source.id]}
              </span>
              <Menu value={order} options={orders} onChange={setOrder} />
            </div>
            <div className="list">
              {items.map((item) => {
                const f = findingOf(item);
                const st = statusOf(item);
                return (
                  <Row
                    key={item.id}
                    open={openRow === item.id}
                    onToggle={() => setOpenRow(openRow === item.id ? null : item.id)}
                    tone={st === "raised" ? "flag" : st === "asked" ? "accent" : "quiet"}
                    lead={<span className="dot" />}
                    title={item.name}
                    detail={rowDetail(source.id, item)}
                    meta={st === "raised" ? "Raised" : st === "asked" ? "With your agent" : st === "cleared" ? "Cleared" : item.meta}
                  >
                    {f && st === "raised" && (
                      <span className="cat-tag" data-cat={categories[f.category].color}>
                        <span className="cat-tile cat-tile--s">
                          <Icon name={categories[f.category].icon} />
                        </span>
                        {categories[f.category].name}
                      </span>
                    )}
                    {f && st === "cleared" && (
                      <p className="t-strong">
                        Cleared by you{state[f.id].note ? <> · “{state[f.id].note}”</> : null}
                      </p>
                    )}
                    <dl className="facts">
                      {item.facts.map(([k, v]) => (
                        <div key={k} style={{ display: "contents" }}>
                          <dt>{k}</dt>
                          <dd>{v.startsWith("~") || v.startsWith("/") ? <CopyButton text={v} tone="quiet" size="s" label={`Copy ${k.toLowerCase()}`} /> : v}</dd>
                        </div>
                      ))}
                    </dl>
                    <div className="row__actions">
                      <CopyButton text={askAbout(source.id, item, f)} tone="quiet" size="s" label="Ask your agent about this" />
                      {f && st !== "cleared" && (
                        <Wipe tone={st === "raised" ? "flag" : "accent"} size="s" onClick={() => onFinding(f)}>
                          Open finding
                          <Icon name="arrow" />
                        </Wipe>
                      )}
                    </div>
                  </Row>
                );
              })}
            </div>
          </>
        ) : (
          <>
            <div className="stack stack--loose">
              <div className="section">
                <span className="t-head">Why it’s empty</span>
                <p>{source.empty!.why}</p>
              </div>
              <div className="section">
                <span className="t-head">What would be here</span>
                <p>{source.empty!.wouldShow}</p>
              </div>
              <div className="section">
                <span className="t-head">Check it yourself</span>
                <p>
                  <span className="quote">{source.empty!.yourself}</span>
                </p>
              </div>
            </div>
            {source.optIn ? (
              <div className="collect-row">
                <Icon name="file" />
                <span className="collect-row__text">
                  <span className="t-strong">Look at new files</span>
                  <span className="muted t-small">macOS asks you once to let EDwaRd see Downloads and Desktop.</span>
                </span>
                <Toggle on={false} label="Look at new files" onChange={() => setFilesOn(true)} />
              </div>
            ) : (
              <div className="collect-row">
                <Icon name="lock" />
                <span className="collect-row__text">
                  <span className="t-strong">Collect {source.name.toLowerCase()}</span>
                  <span className="muted t-small">No collector exists for this. A new source is a code change, not a checkbox.</span>
                </span>
                <Toggle on={false} disabled label={`Collect ${source.name}`} />
              </div>
            )}
          </>
        )}
      </div>
    </>
  );
}
