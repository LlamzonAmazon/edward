import { askSketchy, categories, CategoryId, Finding } from "./data";
import { useSurface } from "./surface";
import { CopyButton, Icon } from "./ui";
import type { FindingState } from "./App";

type Props = {
  state: Record<string, FindingState>;
  onFinding: (f: Finding) => void;
  onClose: () => void;
};

export function AllRaised({ state, onFinding, onClose }: Props) {
  const { sources, findings } = useSurface();
  const raised = findings.filter((f) => state[f.id]?.status === "raised");
  const groups = (Object.keys(categories) as CategoryId[])
    .map((id) => ({ cat: categories[id], items: raised.filter((f) => f.category === id) }))
    .filter((g) => g.items.length > 0);

  return (
    <>
      <div className="dialog__head">
        <span className="icon-box" data-tone="flag">
          <Icon name="shield" />
        </span>
        <h2 className="t-title grow">{raised.length} raised</h2>
        <button className="icon-btn" onClick={onClose} aria-label="Close">
          <Icon name="close" />
        </button>
      </div>
      <div className="dialog__body all-raised">
        {groups.map(({ cat, items }) => (
          <section key={cat.id} className="all-raised__group" data-cat={cat.color}>
            <div className="all-raised__heading">
              <span className="cat-tile">
                <Icon name={cat.icon} />
              </span>
              <span className="t-strong">{cat.name}</span>
              <span className="muted num">{items.length}</span>
            </div>
            <div className="list">
              {items.map((f) => (
                <div key={f.id} className="row" data-tone="flag">
                <button className="row__head" onClick={() => onFinding(f)}>
                  <span className="dot" />
                  <span className="row__text">
                    <span className="row__title">{f.value}</span>
                    <span className="row__detail">{f.facts.join(" · ")}</span>
                  </span>
                  <span className="row__meta">{sources.find((s) => s.id === f.source)!.name}</span>
                  <Icon name="arrow" />
                </button>
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
      <div className="dialog__foot">
        <span className="muted t-small">Your agent reads this same list from the EDwaRd store.</span>
        <span className="spacer" />
        <CopyButton text={askSketchy} label="Ask your agent" />
      </div>
    </>
  );
}
