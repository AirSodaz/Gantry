import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { OverviewPage } from "./OverviewPage";

const mocked = vi.hoisted(() => ({
  api: {
    listWorkspaces: vi.fn(),
    getOverview: vi.fn(),
  },
}));

vi.mock("../../api/ApiProvider", () => ({
  useAdminApi: () => mocked.api,
}));

function renderOverview() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <OverviewPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("OverviewPage", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders overview metrics and attention queue", async () => {
    mocked.api.listWorkspaces.mockResolvedValue({
      items: [{ id: "wsp_dev", display_name: "Development" }],
    });
    mocked.api.getOverview.mockResolvedValue({
      scope: { workspace_id: "", label: "All manageable workspaces" },
      metrics: {
        agents_total: 12,
        published_agents: 8,
        drafts_needing_review: 3,
        invalid_drafts: 1,
        active_runs: 5,
        failed_runs_24_hours: 0,
        awaiting_approvals: 2,
      },
      attention: [
        {
          id: "att_1",
          severity: "high",
          title: "Draft invalid",
          description: "Specification contains schema errors.",
          href: "/agents/agt_1",
        },
      ],
      recent_publications: [
        {
          agent_id: "agt_1",
          agent_name: "Code Reviewer",
          revision_hash: "sha256:abc123def456",
          published_at: "2026-08-17T08:00:00Z",
        },
      ],
      recent_activity: [
        {
          id: "act_1",
          event_type: "agent.published",
          created_at: "2026-08-17T08:00:00Z",
        },
      ],
      unavailable_signals: ["real_time_memory"],
    });

    renderOverview();

    expect(await screen.findByText("Overview")).toBeInTheDocument();
    expect(screen.getByText("Draft invalid")).toBeInTheDocument();
    expect(
      screen.getByText("Specification contains schema errors."),
    ).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("Code Reviewer")).toBeInTheDocument();
  });
});
