import { createSignal, onCleanup, onMount, For, Show } from "solid-js";
import type { Conversation } from "./types";
import ConversationCard from "./components/ConversationCard";

function App() {
  const [conversations, setConversations] = createSignal<Conversation[]>([]);

  onMount(() => {
    fetch("/api/conversations")
      .then((res) => res.json() as Promise<Conversation[]>)
      .then((data) => {
        const sorted = [...data].sort(
          (a, b) =>
            new Date(b.lastEventAt).getTime() -
            new Date(a.lastEventAt).getTime(),
        );
        setConversations(sorted);
      })
      .catch((err: unknown) =>
        console.error("Failed to fetch conversations:", err),
      );

    const es = new EventSource("/api/events");

    es.addEventListener("conversation-update", (event: MessageEvent) => {
      const updated = JSON.parse(event.data as string) as Conversation;
      setConversations((prev) => {
        const idx = prev.findIndex((c) => c.id === updated.id);
        const next = idx === -1 ? [...prev, updated] : prev.map((c, i) => (i === idx ? updated : c));
        return next.sort(
          (a, b) =>
            new Date(b.lastEventAt).getTime() -
            new Date(a.lastEventAt).getTime(),
        );
      });
    });

    onCleanup(() => {
      es.close();
    });
  });

  return (
    <main>
      <h1>Agent Dashboard</h1>
      <Show
        when={conversations().length > 0}
        fallback={<p>No conversations</p>}
      >
        <ul>
          <For each={conversations()}>
            {(conv) => (
              <li>
                <ConversationCard conversation={conv} />
              </li>
            )}
          </For>
        </ul>
      </Show>
    </main>
  );
}

export default App;
