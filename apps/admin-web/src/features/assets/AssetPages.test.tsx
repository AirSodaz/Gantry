import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SkillsPage, ToolsPage } from "./AssetPages";

const mocked = vi.hoisted(() => ({
  api: {
    listWorkspaces: vi.fn(),
    listSkills: vi.fn(),
    listPlugins: vi.fn(),
    listTools: vi.fn(),
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

describe("AssetPages", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders Skills catalog", async () => {
    mocked.api.listWorkspaces.mockResolvedValue({ items: [] });
    mocked.api.listSkills.mockResolvedValue({
      items: [
        {
          id: "skl_1",
          slug: "web-search",
          display_name: "Web Search Skill",
          description: "Enables web search.",
          declared_version: "1.0.0",
          status: "available",
          content_digest: "sha256:11223344556677889900aabbcc",
        },
      ],
    });

    renderWithClient(<SkillsPage />);

    expect(await screen.findByText("Web Search Skill")).toBeInTheDocument();
    expect(screen.getByText(/1\.0\.0 · sha256:11223344/)).toBeInTheDocument();
  });

  it("renders Tools catalog", async () => {
    mocked.api.listTools.mockResolvedValue({
      items: [
        {
          id: "tol_1",
          fully_qualified_name: "gantry.tools.shell",
          version: "2.1.0",
          status: "active",
          content_digest: "sha256:aabbccddeeff00112233445566",
        },
      ],
    });

    renderWithClient(<ToolsPage />);

    expect(await screen.findByText("gantry.tools.shell")).toBeInTheDocument();
    expect(screen.getByText(/2\.1\.0 · sha256:aabbcc/)).toBeInTheDocument();
  });
});
