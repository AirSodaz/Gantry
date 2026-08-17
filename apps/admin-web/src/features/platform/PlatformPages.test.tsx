import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PlatformPage } from "./PlatformPages";
import { PlatformSettingsPage } from "./PlatformSettingsPage";

const mocked = vi.hoisted(() => ({
  api: {
    listPlatformProviders: vi.fn(),
    listRunnerPools: vi.fn(),
    listPlatformCredentials: vi.fn(),
    listDataClassifications: vi.fn(),
    listWorkspaces: vi.fn(),
    getPlatformSettings: vi.fn(),
  },
}));

vi.mock("../../api/ApiProvider", () => ({
  useAdminApi: () => mocked.api,
}));

function renderWithClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("PlatformPages", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders Platform Infrastructure page with tabs and metrics", async () => {
    mocked.api.listPlatformProviders.mockResolvedValue({
      items: [
        {
          id: "prov_1",
          name: "openai-production",
          data_classes: ["internal", "confidential"],
          state: "active",
          credential_reference_id: "cred_1",
        },
      ],
    });
    mocked.api.listRunnerPools.mockResolvedValue({
      items: [
        {
          id: "rpool_1",
          organization_id: "org_1",
          isolation_tier: "gvisor",
          state: "active",
          compatible_protocols: ["gantry.runner/v1"],
          capacity: { max_concurrency: 8 },
        },
      ],
    });
    mocked.api.listPlatformCredentials.mockResolvedValue({ items: [] });
    mocked.api.listDataClassifications.mockResolvedValue({
      items: [
        {
          id: "dc_1",
          label: "Confidential",
          handling: "confidential",
          retention_class: "30_days",
          allowed_provider_ids: ["prov_1"],
          allowed_tool_classes: [],
        },
      ],
    });

    renderWithClient(<PlatformPage />);

    expect(
      await screen.findByText("Platform Infrastructure"),
    ).toBeInTheDocument();
    expect(screen.getByText("openai-production")).toBeInTheDocument();
    expect(screen.getAllByText("Model Providers").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Runner Pools").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Data Classifications").length).toBeGreaterThan(
      0,
    );
  });

  it("renders Platform Settings page with scope switcher and policies", async () => {
    mocked.api.listWorkspaces.mockResolvedValue({ items: [] });
    mocked.api.getPlatformSettings.mockResolvedValue({
      scope: "organization",
      etag: "etag_123",
      validation_state: "valid",
      values: {
        limit_policies: [
          {
            id: "lp_1",
            concurrency: 10,
            duration_seconds: 600,
            output_bytes: 524288,
            etag: "1",
          },
        ],
        environment_profiles: [
          {
            id: "ep_1",
            name: "Production",
            publication_posture: "strict_approval",
            state: "active",
            etag: "1",
          },
        ],
        data_classifications: [],
      },
    });

    renderWithClient(<PlatformSettingsPage />);

    expect(
      await screen.findByText("Platform Settings & Policy Posture"),
    ).toBeInTheDocument();
    expect(screen.getByText("Global Organization Scope")).toBeInTheDocument();
    expect(screen.getByText("Execution Limits & Budgets")).toBeInTheDocument();
    expect(screen.getByText(/512 KB/)).toBeInTheDocument();
  });
});
