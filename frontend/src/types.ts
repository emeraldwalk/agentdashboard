export interface Conversation {
  id: string;
  project: string;
  title: string;
  status: "running" | "waiting_input" | "stopped" | "failed";
  source: "host" | "docker";
  startedAt: string;
  lastEventAt: string;
  isSubagent: boolean;
  parentId: string;
}
