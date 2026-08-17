import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CopilotApiError } from "../../api/client";
import { ApprovalDetailPage } from "./ApprovalDetailPage";

const mocked = vi.hoisted(() => ({
  api: {
    getApproval: vi.fn(),
    decideApproval: vi.fn(),
  },
}));
vi.mock("../../api/ApiProvider", () => ({ useCopilotApi: () => mocked.api }));

const approval = {
  id: "apr_1",
  task_id: "tsk_1",
  run_id: "run_1",
  action_id: "act_1",
  action_digest: "sha256:exact-action",
  approval_revision: 1,
  tool_name: "Directory",
  operation: "update",
  target: "employee/123",
  effect: "write",
  risk_class: "write",
  status: "pending",
  expires_at: "2026-08-20T08:00:00Z",
  action_preview: { effect: "write" },
  policy_version: "policy-v1",
};

function renderDetail() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/approvals/apr_1"]}>
        <Routes>
          <Route
            path="/approvals/:approvalId"
            element={<ApprovalDetailPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ApprovalDetailPage", () => {
  beforeEach(() => vi.clearAllMocks());

  it("decides from the immutable detail with the exact action digest", async () => {
    mocked.api.getApproval.mockResolvedValue(approval);
    mocked.api.decideApproval.mockResolvedValue({
      ...approval,
      status: "satisfied",
      latest_decision: {
        decision: "approve",
        decided_by: "prn_1",
        created_at: "2026-08-16T08:00:00Z",
      },
    });
    const user = userEvent.setup();
    renderDetail();

    expect(await screen.findByText("sha256:exact-action")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Approve action" }));

    await waitFor(() =>
      expect(mocked.api.decideApproval).toHaveBeenCalledWith(
        "apr_1",
        "approve",
        "sha256:exact-action",
        1,
        "",
        expect.any(String),
      ),
    );
  });

  it("replaces stale detail with the server-winning approval state", async () => {
    mocked.api.getApproval.mockResolvedValue(approval);
    mocked.api.decideApproval.mockRejectedValue(
      new CopilotApiError(409, "Approval changed.", "approval_changed", {
        ...approval,
        status: "rejected",
        latest_decision: {
          decision: "reject",
          decided_by: "prn_1",
          created_at: "2026-08-16T08:00:00Z",
        },
      }),
    );
    const user = userEvent.setup();
    renderDetail();

    await user.click(
      await screen.findByRole("button", { name: "Approve action" }),
    );

    expect(await screen.findByText("Rejected")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Approve action" }),
    ).not.toBeInTheDocument();
  });

  it("renders server-winning decision evidence instead of decision controls", async () => {
    mocked.api.getApproval.mockResolvedValue({
      ...approval,
      status: "rejected",
      latest_decision: {
        decision: "reject",
        reason: "Choose a different record.",
        decided_by: "prn_1",
        created_at: "2026-08-16T08:00:00Z",
      },
    });
    renderDetail();

    expect(await screen.findByText("Rejected")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Approve action" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Choose a different record.")).toBeInTheDocument();
  });
});
