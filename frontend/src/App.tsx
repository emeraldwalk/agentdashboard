import { createSignal, onCleanup, onMount, For, Show } from "solid-js";
import type { Session } from "./types";
import SessionCard from "./components/SessionCard";

function App() {
  const [sessions, setSessions] = createSignal<Session[]>([]);

  onMount(() => {
    fetch("/api/sessions")
      .then((res) => res.json() as Promise<Session[]>)
      .then((data) => setSessions(data))
      .catch((err: unknown) => console.error("Failed to fetch sessions:", err));

    const es = new EventSource("/api/events");

    es.addEventListener("session-update", (event: MessageEvent) => {
      const updated = JSON.parse(event.data as string) as Session;
      setSessions((prev) => {
        const idx = prev.findIndex((s) => s.id === updated.id);
        if (idx === -1) {
          return [...prev, updated];
        }
        const next = [...prev];
        next[idx] = updated;
        return next;
      });
    });

    onCleanup(() => {
      es.close();
    });
  });

  return (
    <main>
      <h1>Agent Dashboard</h1>
      <Show when={sessions().length > 0} fallback={<p>No sessions</p>}>
        <ul>
          <For each={sessions()}>
            {(session) => (
              <li>
                <SessionCard session={session} />
              </li>
            )}
          </For>
        </ul>
      </Show>
    </main>
  );
}

export default App;
