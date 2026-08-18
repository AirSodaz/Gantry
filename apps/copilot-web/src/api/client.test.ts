import { describe, expect, it, vi } from "vitest";
import { CopilotApi, CopilotApiError } from "./client";

describe("CopilotApi", () => {
  it("uses Session routes and preserves an ETag on session commands", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: "ses_1" }), { headers: { ETag: '"1"' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: "ses_2" }), { headers: { ETag: '"2"' } }));
    vi.stubGlobal("fetch", fetchMock);
    const api = new CopilotApi(() => "token");
    await api.getSession("ses_1");
    const created = await api.createSession({ agent_id: "agt_1", message: "hello" }, "key");
    expect(fetchMock.mock.calls[0][0]).toContain("/sessions/ses_1");
    expect(fetchMock.mock.calls[1][0]).toContain("/sessions");
    expect(created.conversation_etag).toBe('"2"');
  });

  it("parses a direct CopilotProblem response", async () => {
    const currentResource = { id: "apr_1", state: "rejected" };
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            code: "approval_changed",
            message: "Approval changed.",
            correlation_id: "cor_1",
            retryable: false,
            current_resource: currentResource,
          }),
          { status: 409 },
        ),
      ),
    );
    const api = new CopilotApi(() => "token");

    const error = await api.getApproval("apr_1").catch((reason) => reason);

    expect(error).toBeInstanceOf(CopilotApiError);
    expect(error).toMatchObject({
      status: 409,
      code: "approval_changed",
      message: "Approval changed.",
      correlationId: "cor_1",
      retryable: false,
      currentResource,
    });
  });

  it("uses requester-scoped Agent collections and favorite commands", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [] })))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "agt_1", is_favorite: true })),
      );
    vi.stubGlobal("fetch", fetchMock);
    const api = new CopilotApi(() => "token");

    await api.listAgents("finance", "Operations", "cursor-1", "favorites");
    await api.setAgentFavorite("agt_1", true, "favorite-1");

    expect(fetchMock.mock.calls[0][0]).toContain(
      "/agents?search=finance&category=Operations&cursor=cursor-1&collection=favorites",
    );
    expect(fetchMock.mock.calls[1][0]).toContain("/agents/agt_1/favorite");
    expect(fetchMock.mock.calls[1][1]).toMatchObject({
      method: "PUT",
      headers: expect.objectContaining({ "Idempotency-Key": "favorite-1" }),
      body: JSON.stringify({ is_favorite: true }),
    });
  });
});
