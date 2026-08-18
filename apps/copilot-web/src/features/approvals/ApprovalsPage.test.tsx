import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApprovalsPage } from "./ApprovalsPage";

const mocked = vi.hoisted(() => ({ api: { listApprovals: vi.fn() } }));
vi.mock("../../api/ApiProvider", () => ({ useCopilotApi: () => mocked.api }));

describe("ApprovalsPage", () => {
  beforeEach(() => vi.clearAllMocks());
  it("shows the pending count and links the approval to its session", async () => {
    mocked.api.listApprovals.mockResolvedValue({ items: [{ id: "apr_1", session_id: "ses_1", run_id: "run_1", requester_id: "prn_1", action_id: "act_1", action_digest: "sha256:exact", approval_revision: 1, state: "pending", preview: { summary: "Update employee", tool_display_name: "Directory", operation_display_name: "update", target: "employee/123", effect: "write", risk_class: "write" }, created_at: new Date(Date.now() - 60_000).toISOString(), expires_at: "2026-08-20T08:00:00Z" }], page_info: { has_more: false } });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter><ApprovalsPage /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText("1 action is waiting for your decision.")).toBeInTheDocument();
    expect(screen.getByText("Directory · update")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open session" })).toHaveAttribute("href", "/sessions/ses_1");
  });
});
