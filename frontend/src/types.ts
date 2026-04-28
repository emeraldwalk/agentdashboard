export interface Conversation {
  id: string;
  project: string;
  title: string;
  status: "running" | "waiting_input" | "stopped" | "failed";
  startedAt: string;
  lastEventAt: string;
}
