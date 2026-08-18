import { describe, expect, it } from "vitest";
import { getSessionSubmissionKey } from "./sessionSubmission";

describe("getSessionSubmissionKey", () => {
  it("reuses a key for the same request after a failed network attempt", () => {
    const input = { agent_id: "agt_1", message: "hello" };
    const first = getSessionSubmissionKey(input, null, () => "key-1");
    const retry = getSessionSubmissionKey(input, first, () => "key-2");
    expect(retry.key).toBe("key-1");
  });

  it("creates a new key when the payload changes", () => {
    const first = getSessionSubmissionKey(
      { agent_id: "agt_1", message: "hello" },
      null,
      () => "key-1",
    );
    const changed = getSessionSubmissionKey(
      { agent_id: "agt_1", message: "changed" },
      first,
      () => "key-2",
    );
    expect(changed.key).toBe("key-2");
  });
});
