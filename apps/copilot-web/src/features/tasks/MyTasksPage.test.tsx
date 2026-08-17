import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MyTasksPage } from "./MyTasksPage";

const mocked = vi.hoisted(() => ({
  api: { listTasks: vi.fn(), listAgents: vi.fn() },
}));
vi.mock("../../api/ApiProvider", () => ({ useCopilotApi: () => mocked.api }));

function renderPage(path = "/tasks") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/tasks" element={<MyTasksPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("MyTasksPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocked.api.listAgents.mockResolvedValue({
      items: [{ id: "agt_1", display_name: "Support agent" }],
    });
    mocked.api.listTasks.mockResolvedValue({ items: [] });
  });

  it("hydrates API filters from the shareable task-history URL", async () => {
    renderPage(
      "/tasks?status=completed&agent_id=agt_1&requester_action=input&time_range=7d",
    );

    await waitFor(() =>
      expect(mocked.api.listTasks).toHaveBeenCalledWith(
        expect.objectContaining({
          status: "completed",
          agentId: "agt_1",
          requesterAction: "input",
          createdAfter: expect.any(String),
        }),
      ),
    );
    expect(await screen.findByText("No tasks yet")).toBeInTheDocument();
  });

  it("loads another task page with the server-issued cursor", async () => {
    mocked.api.listTasks
      .mockResolvedValueOnce({
        items: [{ id: "tsk_1", agent_id: "agt_1", status: "completed" }],
        page_info: { has_more: true, next_cursor: "task-page-2" },
      })
      .mockResolvedValueOnce({
        items: [{ id: "tsk_2", agent_id: "agt_1", status: "completed" }],
        page_info: { has_more: false },
      });
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole("button", { name: "Load more" }));
    await waitFor(() =>
      expect(mocked.api.listTasks).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: "task-page-2" }),
      ),
    );
    await waitFor(() => expect(screen.getAllByText("agt_1")).toHaveLength(2));
  });

  it("renders the employee-facing task history projection", async () => {
    mocked.api.listTasks.mockResolvedValue({
      items: [
        {
          id: "tsk_1",
          agent_id: "agt_1",
          agent_display_name: "Support agent",
          title: "Update my account contact",
          status: "awaiting_approval",
          requester_action: "approval",
          created_at: "2026-08-17T01:00:00Z",
          updated_at: "2026-08-17T02:00:00Z",
          artifacts: [{ state: "available", scan_status: "passed" }],
        },
      ],
      page_info: { has_more: false },
    });
    renderPage();

    expect(
      await screen.findByText("Update my account contact"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Approval needed/, { selector: ".task-row-meta" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Artifacts ready/, { selector: ".task-row-meta" }),
    ).toBeInTheDocument();
  });
});
