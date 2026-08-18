import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RunsPage } from "./RunPages";

const mocked = vi.hoisted(() => ({
  api: {
    listWorkspaces: vi.fn(),
    listRuns: vi.fn(),
  },
}));

vi.mock("../../api/ApiProvider", () => ({
  useAdminApi: () => mocked.api,
}));

function renderRuns() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <RunsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("RunPages", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders operational runs table", async () => {
    mocked.api.listWorkspaces.mockResolvedValue({ items: [] });
    mocked.api.listRuns.mockResolvedValue({
      items: [
        {
          id: "run_101",
          session_id: "ses_202",
          workspace_id: "wsp_1",
          workspace_name: "Engineering",
          agent_id: "agt_1",
          agent_name: "Code Reviewer",
          revision_hash: "sha256:abcdef123456",
          requester_id: "prn_1",
          requester_name: "Alice Developer",
          status: "completed",
          session_sequence: 1,
          action_count: 4,
          approval_count: 1,
          created_at: "2026-08-17T08:00:00Z",
        },
      ],
    });

    renderRuns();

    expect(await screen.findByText("run_101")).toBeInTheDocument();
    expect(screen.getByText("Session ses_202")).toBeInTheDocument();
    expect(screen.getByText("Code Reviewer")).toBeInTheDocument();
    expect(screen.getByText("Alice Developer")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
  });
});
