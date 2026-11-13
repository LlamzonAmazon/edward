import { useState } from "react";
import { agentNames, askSketchy, askSurface } from "./data";
import { CopyField, Icon, Swap, token, Wipe } from "./ui";

const agentCount = (n: number) => `${n} ${n === 1 ? "agent" : "agents"}`;
const writeFace = (n: number) => (
  <>
    <Icon name="arrow" />
    Write to {agentCount(n)}
  </>
);
import { AccessList, AgentPicks, Device, SkillPreview } from "./Setup";

type Props = {
  agentIds: string[];
  onAgents: (ids: string[]) => void;
  onClose: () => void;
  onWritten: () => void;
};

const STEPS = 4;

export function FirstRun({ agentIds, onAgents, onClose, onWritten }: Props) {
  const [step, setStep] = useState(0);
  const [dir, setDir] = useState<"next" | "back">("next");
  const [written, setWritten] = useState(false);
  const go = (n: number) => {
    setDir(n > step ? "next" : "back");
    setStep(n);
  };
  const write = () => {
    setWritten(true);
    onWritten();
    window.setTimeout(() => go(3), token("--dur-hold") / 2);
  };

  const titles = ["What do you use?", "What EDwaRd can see", `Add EDwaRd to ${agentNames(agentIds, " and ")}`, "Ask your agent"];

  return (
    <>
      <div className="dialog__head">
        <h2 className="t-title grow">
          <Swap swapKey={step}>{titles[step]}</Swap>
        </h2>
        <button className="icon-btn" onClick={onClose} aria-label="Close">
          <Icon name="close" />
        </button>
      </div>
      <div className="dialog__body">
        <div className="step stack stack--loose" key={step} data-dir={dir}>
          {step === 0 && <AgentPicks agentIds={agentIds} onAgents={onAgents} />}
          {step === 1 && (
            <>
              <Device />
              <AccessList />
            </>
          )}
          {step === 2 && (
            <>
              <p className="muted">EDwaRd adds one skill file. Your agent loads it when you ask about this Mac. EDwaRd does not edit your agent’s other settings or instruction files.</p>
              <SkillPreview agentIds={agentIds} />
            </>
          )}
          {step === 3 && (
            <>
              <CopyField label="Find what’s sketchy" text={askSketchy} />
              <CopyField label="Or map the surface" text={askSurface} tone="quiet" />
            </>
          )}
        </div>
      </div>
      <div className="dialog__foot">
        <span className="steps" aria-label={`Step ${step + 1} of ${STEPS}`}>
          {Array.from({ length: STEPS }, (_, i) => (
            <span key={i} data-on={i === step} />
          ))}
        </span>
        <span className="spacer" />
        {step > 0 && step < 3 && (
          <Wipe tone="quiet" onClick={() => go(step - 1)}>
            Back
          </Wipe>
        )}
        {step === 0 && (
          <Wipe tone="quiet" onClick={onClose}>
            Skip
          </Wipe>
        )}
        {step < 2 && (
          <Wipe tone="accent" disabled={step === 0 && !agentIds.length} onClick={() => go(step + 1)}>
            Next
            <Icon name="arrow" />
          </Wipe>
        )}
        {step === 2 && (
          <Wipe
            tone="accent"
            swapKey="write"
            reserve={[writeFace(1), writeFace(7)]}
            disabled={!agentIds.length}
            onClick={write}
            done={written ? "ok" : null}
            doneLabel={`Wrote to ${agentCount(agentIds.length)}`}
          >
            {writeFace(agentIds.length)}
          </Wipe>
        )}
        {step === 3 && (
          <Wipe tone="accent" onClick={onClose}>
            <Icon name="check" />
            Done
          </Wipe>
        )}
      </div>
    </>
  );
}
