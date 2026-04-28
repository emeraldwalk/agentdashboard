import { createSignal, onCleanup, onMount } from "solid-js";
import type { Conversation } from "../types";

interface Props {
  conversation: Conversation;
}

const STATUS_COLORS: Record<Conversation["status"], string> = {
  running: "green",
  waiting_input: "yellow",
  stopped: "gray",
  failed: "red",
};

function relativeTime(isoString: string): string {
  const diffMs = Date.now() - new Date(isoString).getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) {
    return "just now";
  }

  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) {
    return `${diffMin} min ago`;
  }

  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) {
    return `${diffHr} hr ago`;
  }

  const diffDays = Math.floor(diffHr / 24);
  return `${diffDays} day${diffDays === 1 ? "" : "s"} ago`;
}

function ConversationCard(props: Props) {
  const [now, setNow] = createSignal(Date.now());

  onMount(() => {
    const timer = setInterval(() => setNow(Date.now()), 30_000);
    onCleanup(() => clearInterval(timer));
  });

  const badgeStyle = () => {
    const color = STATUS_COLORS[props.conversation.status];
    return `background-color: ${color}; color: white; padding: 2px 8px; border-radius: 4px;`;
  };

  const lastSeen = () => {
    void now();
    return relativeTime(props.conversation.lastEventAt);
  };

  const displayName = () =>
    props.conversation.title || props.conversation.id.slice(0, 8);

  return (
    <div>
      <small>{props.conversation.project}</small>
      <strong>{displayName()}</strong>
      <span style={badgeStyle()}>{props.conversation.status}</span>
      <span>{lastSeen()}</span>
    </div>
  );
}

export default ConversationCard;
