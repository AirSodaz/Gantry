import type { CreateSessionInput } from "../../api/types";

export type PendingSubmission = { key: string; signature: string };

export function getSessionSubmissionKey(
  input: CreateSessionInput,
  pending: PendingSubmission | null,
  createKey: () => string = () => crypto.randomUUID(),
) {
  const signature = JSON.stringify(input);
  if (pending?.signature === signature) return { key: pending.key, signature };
  return { key: createKey(), signature };
}
