import type { SubmitTaskInput } from '../../api/types';

export type PendingSubmission = { key: string; signature: string };

export function getSubmissionKey(input: SubmitTaskInput, pending: PendingSubmission | null, createKey: () => string = () => crypto.randomUUID()) {
  const signature = JSON.stringify(input);
  if (pending?.signature === signature) return { key: pending.key, signature };
  return { key: createKey(), signature };
}
