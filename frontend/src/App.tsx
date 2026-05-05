import { createSignal, createMemo, onCleanup, onMount, For, Show } from "solid-js";
import type { Conversation } from "./types";
import ProjectCard from "./components/ProjectCard";
import type { ProjectGroup } from "./components/ProjectCard";

function App() {
  const [conversations, setConversations] = createSignal<Conversation[]>([]);

  const projectGroups = createMemo<ProjectGroup[]>(() => {
    const map = new Map<string, Conversation[]>();
    for (const c of conversations()) {
      const list = map.get(c.project) ?? [];
      list.push(c);
      map.set(c.project, list);
    }
    const groups: ProjectGroup[] = [];
    for (const [project, convs] of map) {
      const sorted = convs.toSorted(
        (a, b) => new Date(b.lastEventAt).getTime() - new Date(a.lastEventAt).getTime(),
      );
      groups.push({
        project,
        lastEventAt: sorted[0].lastEventAt,
        conversations: sorted,
      });
    }
    return groups.toSorted(
      (a, b) => new Date(b.lastEventAt).getTime() - new Date(a.lastEventAt).getTime(),
    );
  });

  onMount(() => {
    fetch("/api/conversations")
      .then((res) => res.json() as Promise<Conversation[]>)
      .then((data) => {
        setConversations(data);
      })
      .catch((err: unknown) => console.error("Failed to fetch conversations:", err));

    const es = new EventSource("/api/events");

    es.addEventListener("conversation-update", (event: MessageEvent) => {
      const updated = JSON.parse(event.data as string) as Conversation;
      setConversations((prev) => {
        const idx = prev.findIndex((c) => c.id === updated.id);
        return idx === -1 ? [...prev, updated] : prev.map((c, i) => (i === idx ? updated : c));
      });
    });

    onCleanup(() => {
      es.close();
    });
  });

  return (
    <main>
      <h1>Agent Dashboard</h1>
      <Show when={projectGroups().length > 0} fallback={<p>No conversations</p>}>
        <For each={projectGroups()}>{(group) => <ProjectCard group={group} />}</For>
      </Show>
    </main>
  );
}

export default App;
