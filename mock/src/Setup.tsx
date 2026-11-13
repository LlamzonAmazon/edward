import { useState } from "react";
import { access, filesPrompt, skillBody } from "./data";
import { openPrivacy } from "./api";
import { useSurface } from "./surface";
import { Icon, Toggle, Wipe } from "./ui";

export function AgentPicks({ agentIds, onAgents }: { agentIds: string[]; onAgents: (ids: string[]) => void }) {
  const { agents } = useSurface();
  return (
    <div className="agents">
      {agents.map((a) => (
        <label key={a.id} className="agent-pick">
          <span className="agent-pick__name">
            {a.name}
            <span className="agent-pick__where">{a.found ? `Found ${a.found}` : "Not on this laptop"}</span>
          </span>
          <Toggle
            on={agentIds.includes(a.id)}
            label={a.name}
            onChange={(v) => onAgents(v ? [...agentIds, a.id] : agentIds.filter((id) => id !== a.id))}
          />
        </label>
      ))}
    </div>
  );
}

export function Device() {
  const { host } = useSurface();
  return (
    <div className="device">
      <span className="icon-box" data-tone="accent">
        <Icon name="laptop" />
      </span>
      <span className="device__text">
        <span className="t-strong">{host.name}</span>
        <span className="muted t-small">
          {host.os} · as {host.user} · read from this Mac
        </span>
      </span>
    </div>
  );
}

export function AccessList() {
  const [opened, setOpened] = useState<string[]>([]);
  const { filesOn, setFilesOn, sources } = useSurface();
  return (
    <div className="access">
      {sources
        .filter((s) => s.collected && !s.optIn)
        .map((s) => (
          <div key={s.id} className="access__row">
            <Icon name={s.icon} />
            <span className="access__text">
              <span className="t-strong">{s.name}</span>
            </span>
            <span className="access__state" data-on="true">
              <Icon name="check" />
              Seen as you
            </span>
          </div>
        ))}
      <div className="access__row">
        <Icon name="file" />
        <span className="access__text">
          <span className="t-strong">New files</span>
          <span className="muted t-small">{filesOn ? "On. Downloads, Desktop, and /tmp, names only." : filesPrompt}</span>
        </span>
        <Toggle on={filesOn} label="New files" onChange={setFilesOn} />
      </div>
      {access.map((a) => (
        <div key={a.id} className="access__row">
          <Icon name="lock" />
          <span className="access__text">
            <span className="t-strong">{a.unlocks}</span>
            <span className="muted t-small">{a.how === "settings" ? `Needs ${a.name}. You turn it on in ${a.where}.` : a.where}</span>
          </span>
          {a.how === "settings" ? (
            <Wipe
              tone="quiet"
              size="s"
              done={opened.includes(a.id)}
              swapKey={opened.includes(a.id) ? "waiting" : "open"}
              onClick={() => {
                setOpened([...opened, a.id]);
                void openPrivacy("fda").catch(() => {});
              }}
            >
              <Icon name={opened.includes(a.id) ? "check" : "arrow"} />
              {opened.includes(a.id) ? "Waiting for the switch" : "Open System Settings"}
            </Wipe>
          ) : (
            <span className="access__state">Not in v1</span>
          )}
        </div>
      ))}
    </div>
  );
}

export function SkillPreview({ agentIds }: { agentIds: string[] }) {
  const { agents } = useSurface();
  // Codex, Cursor, OpenCode and OpenClaw share one file, so list each path once.
  const paths = [...new Set(agents.filter((a) => agentIds.includes(a.id)).map((a) => a.skill))];
  if (paths.length === 0) return null;
  return (
    <div className="preview">
      {paths.map((p) => (
        <span key={p} className="preview__file">
          {p}
        </span>
      ))}
      {skillBody.map((line, i) => (
        <span key={i}>{line}</span>
      ))}
    </div>
  );
}
