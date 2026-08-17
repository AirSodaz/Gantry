import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PoliciesPage } from "./PolicyPages";

const mocked = vi.hoisted(() => ({
  api: {
    listPolicies: vi.fn(),
    createPolicy: vi.fn(),
  },
}));

vi.mock("../../api/ApiProvider", () => ({
  useAdminApi: () => mocked.api,
}));

function renderPolicies() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <PoliciesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("PolicyPages", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders policy list", async () => {
    mocked.api.listPolicies.mockResolvedValue({
      items: [
        {
          id: "plc_1",
          name: "Strict Approvals",
          type: "approval",
          state: "active",
          active_binding_count: 3,
          draft_etag: "etag_1",
        },
      ],
    });

    renderPolicies();

    expect(await screen.findByText("Strict Approvals")).toBeInTheDocument();
    expect(screen.getByText("plc_1")).toBeInTheDocument();
  });
});
