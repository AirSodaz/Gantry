import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SessionPage } from "./SessionPage";

const mocked = vi.hoisted(() => ({ api: { getSession: vi.fn(), createSessionEventsTicket: vi.fn(), cancelSessionRun: vi.fn(), retrySessionRun: vi.fn(), appendSessionMessage: vi.fn(), listSessionRuns: vi.fn(), listApprovals: vi.fn() } }));
vi.mock("../../api/ApiProvider", () => ({ useCopilotApi: () => mocked.api }));
const baseSession = { id: "ses_1", owner_principal_id: "prn_1", mode: "private", agent: { id: "agt_1", display_name: "Lifecycle Demo", revision: { id: "rev_1", hash: "abc", message: "Production" } }, state: "active", conversation_revision: 1, queued_run_count: 0, members: [], messages: [], created_at: "2026-08-16T08:00:00Z", updated_at: "2026-08-16T08:00:00Z" };
function renderSession() { const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={["/sessions/ses_1"]}><Routes><Route path="/sessions/:sessionId" element={<SessionPage />} /></Routes></MemoryRouter></QueryClientProvider>); }

describe("SessionPage", () => {
  beforeEach(() => { vi.clearAllMocks(); MockWebSocket.instances = []; mocked.api.listSessionRuns.mockResolvedValue({ items: [], page_info: { has_more: false } }); mocked.api.createSessionEventsTicket.mockResolvedValue({ ticket: "evt.test", session_id: "ses_1", websocket_url: "wss://stream.example.test/events", expires_at: "2026-08-16T08:01:00Z" }); });
  afterEach(() => vi.unstubAllGlobals());
  it("cancels the active run through its fenced session route", async () => { mocked.api.getSession.mockResolvedValue({ ...baseSession, executing_run: { id: "run_1", session_sequence: 1, requester_id: "prn_1", state: "running", created_at: "2026-08-16T08:00:00Z" } }); mocked.api.cancelSessionRun.mockResolvedValue({ id: "run_1", session_sequence: 1, requester_id: "prn_1", state: "canceling", created_at: "2026-08-16T08:00:00Z" }); renderSession(); await userEvent.setup().click(await screen.findByRole("button", { name: "Cancel run" })); await waitFor(() => expect(mocked.api.cancelSessionRun).toHaveBeenCalledWith("ses_1", "run_1", expect.any(String))); });
  it("continues a requester-input run with an ETag", async () => { mocked.api.getSession.mockResolvedValue({ ...baseSession, conversation_etag: '"4"' }); mocked.api.listSessionRuns.mockResolvedValue({ items: [{ id: "run_1", session_sequence: 1, requester_id: "prn_1", state: "failed", outcome: "requester_input_required", created_at: "2026-08-16T08:00:00Z" }], page_info: { has_more: false } }); mocked.api.appendSessionMessage.mockResolvedValue(baseSession); renderSession(); const user = userEvent.setup(); await user.type(await screen.findByLabelText("Continue this session"), "Use a different target"); await user.click(screen.getByRole("button", { name: "Continue" })); await waitFor(() => expect(mocked.api.appendSessionMessage).toHaveBeenCalledWith("ses_1", { message: "Use a different target" }, expect.any(String), '"4"')); });
  it("deduplicates session events and keeps streaming after a terminal run", async () => { vi.stubGlobal("WebSocket", MockWebSocket); mocked.api.getSession.mockResolvedValue(baseSession); renderSession(); await waitFor(() => expect(MockWebSocket.instances).toHaveLength(1)); const event = { schema_version: "gantry.copilot.event/v1", session_id: "ses_1", run_id: "run_1", session_sequence: 1, run_sequence: 1, cursor: "cur_1", event: { type: "content_segment", message_id: "msg_1", segment_index: 0, text: "Hello" } }; MockWebSocket.instances[0].emit(event); MockWebSocket.instances[0].emit(event); expect(await screen.findByText("Hello")).toBeInTheDocument(); MockWebSocket.instances[0].emit({ ...event, session_sequence: 2, event: { type: "run_state_changed", run: { id: "run_1", session_sequence: 2, requester_id: "prn_1", state: "completed", created_at: "2026-08-16T08:00:00Z" } } }); expect(MockWebSocket.instances).toHaveLength(1); });
  it("atomically replaces the projection when the cursor expires", async () => { vi.stubGlobal("WebSocket", MockWebSocket); mocked.api.getSession.mockResolvedValue(baseSession); renderSession(); await waitFor(() => expect(MockWebSocket.instances).toHaveLength(1)); MockWebSocket.instances[0].emit({ type: "cursor_expired", snapshot: { schema_version: "gantry.copilot.snapshot/v1", cursor: "fresh", session: { ...baseSession, title: "Fresh projection" }, runs: [], approvals: [] } }); expect(await screen.findByRole("heading", { name: "Fresh projection" })).toBeInTheDocument(); expect(screen.getByRole("status")).toHaveTextContent("Earlier live history expired"); });
});

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  constructor(readonly url: string) { MockWebSocket.instances.push(this); queueMicrotask(() => this.onopen?.(new Event("open"))); }
  close() { this.onclose?.(new Event("close")); }
  emit(frame: unknown) { this.onmessage?.(new MessageEvent("message", { data: JSON.stringify(frame) })); }
}
