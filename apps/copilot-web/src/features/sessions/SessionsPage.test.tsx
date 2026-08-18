import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SessionsPage } from "./SessionsPage";

const mocked = vi.hoisted(() => ({ api: { listSessions: vi.fn(), listAgents: vi.fn() } }));
vi.mock("../../api/ApiProvider", () => ({ useCopilotApi: () => mocked.api }));
function renderPage(path = "/sessions?status=active&agent_id=agt_1&requester_action=approval") { const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><SessionsPage /></MemoryRouter></QueryClientProvider>); }

describe("SessionsPage", () => {
  beforeEach(() => { vi.clearAllMocks(); mocked.api.listAgents.mockResolvedValue({ items: [] }); });
  it("hydrates API filters from the shareable session-history URL", async () => { mocked.api.listSessions.mockResolvedValue({ items: [], page_info: { has_more: false } }); renderPage(); await waitFor(() => expect(mocked.api.listSessions).toHaveBeenCalledWith(expect.objectContaining({ state: "active", agentId: "agt_1", myAction: "approval" }))); });
  it("offers only Session states and supported requester actions", async () => { mocked.api.listSessions.mockResolvedValue({ items: [], page_info: { has_more: false } }); renderPage("/sessions"); expect(await screen.findByText("No sessions yet")).toBeInTheDocument(); expect(screen.getByText("Active")).toBeInTheDocument(); expect(screen.getByText("Archived")).toBeInTheDocument(); expect(screen.queryByText("Needs input")).not.toBeInTheDocument(); });
  it("loads another session page with the server-issued cursor", async () => { mocked.api.listSessions.mockResolvedValueOnce({ items: [], page_info: { has_more: true, next_cursor: "next" } }).mockResolvedValueOnce({ items: [], page_info: { has_more: false } }); renderPage("/sessions"); await screen.findByRole("button", { name: "Load more" }); screen.getByRole("button", { name: "Load more" }).click(); await waitFor(() => expect(mocked.api.listSessions).toHaveBeenLastCalledWith(expect.objectContaining({ cursor: "next" }))); });
  it("renders the employee-facing session history projection", async () => { mocked.api.listSessions.mockResolvedValue({ items: [{ id: "ses_1", owner_principal_id: "prn_1", mode: "private", agent: { id: "agt_1", display_name: "Directory agent", revision: { id: "rev_1", hash: "abc", message: "Production" } }, title: "Update my account contact", state: "active", conversation_revision: 1, my_action: "none", queued_run_count: 0, members: [], messages: [], created_at: "2026-08-16T08:00:00Z", updated_at: "2026-08-16T08:00:00Z" }], page_info: { has_more: false } }); renderPage("/sessions"); expect(await screen.findByText("Update my account contact")).toBeInTheDocument(); expect(screen.getByRole("link", { name: /directory agent/i })).toHaveAttribute("href", "/sessions/ses_1"); });
});
