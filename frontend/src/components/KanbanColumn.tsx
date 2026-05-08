import { For, Show } from "solid-js";
import ProjectCard from "./ProjectCard";
import type { ProjectGroup } from "./ProjectCard";

interface KanbanColumnProps {
  title: string;
  groups: ProjectGroup[];
}

function KanbanColumn(props: KanbanColumnProps) {
  return (
    <div style="flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px;">
      <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px;">
        <h2 style="margin: 0; font-size: 1em; font-weight: 600;">{props.title}</h2>
        <span style="font-size: 0.75em; background: #333; color: #aaa; border-radius: 10px; padding: 1px 7px;">
          {props.groups.length}
        </span>
      </div>
      <Show
        when={props.groups.length > 0}
        fallback={<p style="color: #555; font-size: 0.85em; margin: 0;">No sessions</p>}
      >
        <For each={props.groups}>{(group) => <ProjectCard group={group} />}</For>
      </Show>
    </div>
  );
}

export default KanbanColumn;
