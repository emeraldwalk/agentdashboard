import { createSignal, createMemo, onCleanup, onMount } from "solid-js";
import type { Conversation } from "./types";
import KanbanColumn from "./components/KanbanColumn";
import type { ProjectGroup } from "./components/ProjectCard";

function conversationColumn(conv: Conversation, now: number): "pending" | "done" | "archived" {
  const age = now - new Date(conv.lastEventAt).getTime();
  if (age > 48 * 60 * 60 * 1000) return "archived";
  if (conv.status === "running" || conv.status === "waiting_input") return "pending";
  return "done";
}

function buildColumnGroups(columnConvs: Conversation[]): ProjectGroup[] {
  const map = new Map<string, Conversation[]>();
  for (const c of columnConvs) {
    const list = map.get(c.project) ?? [];
    list.push(c);
    map.set(c.project, list);
  }
  const groups: ProjectGroup[] = [];
  for (const [project, convs] of map) {
    const sorted = convs.toSorted(
      (a, b) => new Date(b.lastEventAt).getTime() - new Date(a.lastEventAt).getTime(),
    );
    groups.push({ project, lastEventAt: sorted[0].lastEventAt, conversations: sorted });
  }
  return groups.toSorted(
    (a, b) => new Date(b.lastEventAt).getTime() - new Date(a.lastEventAt).getTime(),
  );
}

function App() {
  const [conversations, setConversations] = createSignal<Conversation[]>([]);

  const columnGroups = createMemo(() => {
    const now = Date.now();
    const pending: Conversation[] = [];
    const done: Conversation[] = [];
    const archived: Conversation[] = [];
    for (const c of conversations()) {
      const col = conversationColumn(c, now);
      if (col === "pending") pending.push(c);
      else if (col === "done") done.push(c);
      else archived.push(c);
    }
    return {
      pending: buildColumnGroups(pending),
      done: buildColumnGroups(done),
      archived: buildColumnGroups(archived),
    };
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
      <div style="display: flex; gap: 16px; align-items: flex-start;">
        <KanbanColumn title="Pending" groups={columnGroups().pending} />
        <KanbanColumn title="Done" groups={columnGroups().done} />
        <KanbanColumn title="Archived" groups={columnGroups().archived} />
      </div>
    </main>
  );
}

export default App;
