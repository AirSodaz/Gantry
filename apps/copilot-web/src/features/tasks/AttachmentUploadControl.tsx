import { useEffect, useMemo, useState } from "react";
import {
  CircleCheck,
  LoaderCircle,
  Paperclip,
  ShieldAlert,
  X,
} from "lucide-react";
import { useCopilotApi } from "../../api/ApiProvider";
import type { Attachment } from "../../api/types";

type PendingAttachment = {
  localId: string;
  filename: string;
  sizeBytes: number;
  progress: number;
  state: "hashing" | "uploading" | "validating" | "available" | "error";
  attachment?: Attachment;
  error?: string;
};

export type AttachmentUploadState = {
  attachmentIDs: string[];
  hasPending: boolean;
};

export function AttachmentUploadControl({
  disabled = false,
  onChange,
}: {
  disabled?: boolean;
  onChange: (state: AttachmentUploadState) => void;
}) {
  const api = useCopilotApi();
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const attachmentIDs = useMemo(
    () =>
      attachments.flatMap((item) =>
        item.state === "available" && item.attachment
          ? [item.attachment.id]
          : [],
      ),
    [attachments],
  );
  const hasPending = attachments.some(
    (item) => !["available", "error"].includes(item.state),
  );

  useEffect(
    () => onChange({ attachmentIDs, hasPending }),
    [attachmentIDs, hasPending, onChange],
  );

  const updateAttachment = (
    localId: string,
    patch: Partial<PendingAttachment>,
  ) => {
    setAttachments((current) =>
      current.map((item) =>
        item.localId === localId ? { ...item, ...patch } : item,
      ),
    );
  };

  const selectAttachments = async (files: FileList | null) => {
    if (!files) return;
    await Promise.all(
      Array.from(files).map(async (file) => {
        const localId = crypto.randomUUID();
        setAttachments((current) => [
          ...current,
          {
            localId,
            filename: file.name,
            sizeBytes: file.size,
            progress: 0,
            state: "hashing",
          },
        ]);
        try {
          if (file.size > 64 * 1024 * 1024)
            throw new Error("Files must be 64 MB or smaller.");
          const attachment = await api.createAttachment({
            filename: file.name,
            media_type: file.type || "application/octet-stream",
            size_bytes: file.size,
            digest: await digestFile(file),
            classification: "internal",
          });
          updateAttachment(localId, { state: "uploading", attachment });
          await api.uploadAttachment(attachment, file, (progress) =>
            updateAttachment(localId, { progress }),
          );
          updateAttachment(localId, { state: "validating", progress: 100 });
          const completed = await api.completeAttachment(attachment.id);
          if (
            completed.state !== "available" ||
            completed.scan_status !== "passed"
          ) {
            throw new Error("The attachment is still being scanned.");
          }
          updateAttachment(localId, {
            state: "available",
            attachment: completed,
          });
        } catch (error) {
          updateAttachment(localId, {
            state: "error",
            error:
              error instanceof Error
                ? error.message
                : "Attachment upload failed.",
          });
        }
      }),
    );
  };

  return (
    <>
      {attachments.length ? (
        <ul className="attachment-list" aria-label="Selected attachments">
          {attachments.map((attachment) => (
            <li
              key={attachment.localId}
              className={`attachment-item attachment-${attachment.state}`}
            >
              {attachment.state === "available" ? (
                <CircleCheck size={15} aria-hidden="true" />
              ) : attachment.state === "error" ? (
                <ShieldAlert size={15} aria-hidden="true" />
              ) : (
                <LoaderCircle
                  size={15}
                  aria-hidden="true"
                  className="ds-spin"
                />
              )}
              <span className="attachment-name">{attachment.filename}</span>
              <span className="attachment-status">
                {attachment.state === "hashing"
                  ? "Preparing"
                  : attachment.state === "uploading"
                    ? `${attachment.progress}%`
                    : attachment.state === "validating"
                      ? "Validating"
                      : attachment.state === "available"
                        ? "Ready"
                        : attachment.error}
              </span>
              <button
                type="button"
                className="ds-icon-button ds-icon-button-sm attachment-remove"
                onClick={() =>
                  setAttachments((current) =>
                    current.filter(
                      (item) => item.localId !== attachment.localId,
                    ),
                  )
                }
                aria-label={`Remove ${attachment.filename}`}
                title={`Remove ${attachment.filename}`}
                disabled={disabled}
              >
                <X size={14} />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      <label className="attachment-add" title="Add attachment">
        <Paperclip size={15} aria-hidden="true" />
        <span className="sr-only">Add attachment</span>
        <input
          type="file"
          multiple
          disabled={disabled}
          onChange={(event) => {
            void selectAttachments(event.target.files);
            event.currentTarget.value = "";
          }}
        />
      </label>
    </>
  );
}

async function digestFile(file: File) {
  if (!crypto.subtle)
    throw new Error("This browser cannot securely prepare attachments.");
  const data = await file.arrayBuffer();
  const digest = await crypto.subtle.digest("SHA-256", data);
  return `sha256:${Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}
