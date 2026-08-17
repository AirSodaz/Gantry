import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuditPage } from "./AuditPages";

const mocked = vi.hoisted(() => ({
  api: {
    listWorkspaces: vi.fn(),
    listAuditEvents: vi.fn(),
    createAuditExport: vi.fn(),
  },
}));

vi.mock("../../api/ApiProvider", () => ({
  useAdminApi: () => mocked.api,
}));

function renderAudit() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AuditPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AuditPages", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders immutable audit log table", async () => {
    mocked.api.listWorkspaces.mockResolvedValue({ items: [] });
    mocked.api.listAuditEvents.mockResolvedValue({
      items: [
        {
          id: "aud_1",
          event_type: "agent.published",
          actor_id: "prn_admin",
          actor_name: "Admin User",
          resource_type: "agent",
          resource_id: "agt_1",
          scope: "Engineering",
          outcome: "success",
          risk: "medium",
          created_at: "2026-08-17T08:00:00Z",
          redaction_metadata: { mode: "none", redacted_fields: [] },
          payload: { action: "publish" },
        },
      ],
      page_info: { has_more: false },
    });

    renderAudit();

    expect(await screen.findByText("Audit")).toBeInTheDocument();
    expect(screen.getByText("agent.published")).toBeInTheDocument();
    expect(screen.getByText("Admin User")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
  });
});
