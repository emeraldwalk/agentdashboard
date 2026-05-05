import { createSignal, For, Show } from "solid-js";
import type { Conversation } from "../types";
import ConversationRow from "./ConversationRow";

export interface ProjectGroup {
  project: string;
  lastEventAt: string;
  conversations: Conversation[];
}

interface Props {
  group: ProjectGroup;
}

const COLLAPSED_LIMIT = 3;

function ProjectCard(props: Props) {
  const [expanded, setExpanded] = createSignal(false);

  const visible = () =>
    expanded() ? props.group.conversations : props.group.conversations.slice(0, COLLAPSED_LIMIT);

  const hiddenCount = () => Math.max(0, props.group.conversations.length - COLLAPSED_LIMIT);

  return (
    <div style="border: 1px solid #333; border-radius: 6px; padding: 10px 14px; margin-bottom: 10px;">
      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px;">
        <strong style="font-size: 0.95em;">{props.group.project}</strong>
        <Show when={hiddenCount() > 0}>
          <button
            onClick={() => setExpanded((v) => !v)}
            style="font-size: 0.75em; background: none; border: none; color: #888; cursor: pointer; padding: 0;"
          >
            {expanded() ? "Show less" : `Show all ${props.group.conversations.length}`}
          </button>
        </Show>
      </div>
      <For each={visible()}>{(conv) => <ConversationRow conversation={conv} />}</For>
    </div>
  );
}

export default ProjectCard;
