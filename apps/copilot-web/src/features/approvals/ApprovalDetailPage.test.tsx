import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CopilotApiError } from "../../api/client";
import { ApprovalDetailPage } from "./ApprovalDetailPage";

const mocked = vi.hoisted(() => ({ api: { getApproval: vi.fn(), decideApproval: vi.fn() } }));
vi.mock("../../api/ApiProvider", () => ({ useCopilotApi: () => mocked.api }));
const approval = { id: "apr_1", session_id: "ses_1", run_id: "run_1", requester_id: "prn_1", action_id: "act_1", action_digest: "sha256:exact-action", approval_revision: 1, state: "pending", preview: { summary: "Update employee", tool_display_name: "Directory", operation_display_name: "update", target: "employee/123", effect: "write", risk_class: "write" }, expires_at: "2026-08-20T08:00:00Z", created_at: "2026-08-16T08:00:00Z" };
function renderDetail() { const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={["/approvals/apr_1"]}><Routes><Route path="/approvals/:approvalId" element={<ApprovalDetailPage />} /></Routes></MemoryRouter></QueryClientProvider>); }
describe("ApprovalDetailPage", () => {
  beforeEach(() => vi.clearAllMocks());
  it("decides from the immutable detail with the exact action digest", async () => { mocked.api.getApproval.mockResolvedValue(approval); mocked.api.decideApproval.mockResolvedValue({ ...approval, state: "approved", decision: { decision: "approve", decided_at: "2026-08-16T08:01:00Z" } }); renderDetail(); await userEvent.setup().click(await screen.findByRole("button", { name: "Approve action" })); await waitFor(() => expect(mocked.api.decideApproval).toHaveBeenCalledWith("apr_1", "approve", "sha256:exact-action", 1, "", expect.any(String))); });
  it("uses a server-winning approval state after a conflict", async () => { mocked.api.getApproval.mockResolvedValue(approval); mocked.api.decideApproval.mockRejectedValue(new CopilotApiError(409, "Approval changed.", "approval_changed", { ...approval, state: "rejected", decision: { decision: "reject", reason: "Choose a different record.", decided_at: "2026-08-16T08:01:00Z" } })); renderDetail(); await userEvent.setup().click(await screen.findByRole("button", { name: "Approve action" })); expect(await screen.findByText("Rejected")).toBeInTheDocument(); });
});
