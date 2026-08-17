import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AgentsPage } from "./AgentsPage";

const mocked = vi.hoisted(() => ({
  api: {
    listWorkspaces: vi.fn(),
    listAgents: vi.fn(),
  },
}));

vi.mock("../../api/ApiProvider", () => ({
  useAdminApi: () => mocked.api,
}));

function renderAgents() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AgentsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AgentsPage", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders agent list with status and categories", async () => {
    mocked.api.listWorkspaces.mockResolvedValue({
      items: [{ id: "wsp_1", display_name: "Engineering" }],
    });
    mocked.api.listAgents.mockResolvedValue({
      items: [
        {
          id: "agt_1",
          slug: "support-bot",
          display_name: "Customer Support Bot",
          description: "Handles customer support queries.",
          category: "Support",
          lifecycle_status: "published",
        },
      ],
    });

    renderAgents();

    expect(await screen.findByText("Customer Support Bot")).toBeInTheDocument();
    expect(screen.getByText(/Support · support-bot/)).toBeInTheDocument();
  });

  it("renders empty state when no agents match", async () => {
    mocked.api.listWorkspaces.mockResolvedValue({ items: [] });
    mocked.api.listAgents.mockResolvedValue({ items: [] });

    renderAgents();

    expect(
      await screen.findByText("No agents in this scope"),
    ).toBeInTheDocument();
    expect(screen.getByText("Create agent")).toBeInTheDocument();
  });
});
