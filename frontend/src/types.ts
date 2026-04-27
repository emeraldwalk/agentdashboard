export interface Session {
  id: string;
  agentName: string;
  status: "running" | "waiting_input" | "stopped" | "failed";
  startedAt: string; // ISO 8601
  lastEventAt: string; // ISO 8601
}
