import { useEffect, useRef, useState } from "react";
import type { QueryClient } from "@tanstack/react-query";
import type { CopilotApi } from "../../api/client";
import type {
  Session,
  SessionEventFrame,
  SessionEventSnapshot,
} from "../../api/types";

type StreamState = "connecting" | "connected" | "reconnecting" | "closed";
type CursorExpiredFrame = {
  type: "cursor_expired";
  snapshot: SessionEventSnapshot;
};
type SnapshotFrame = SessionEventSnapshot & { type?: "snapshot" };
type StreamFrame = SessionEventFrame | SnapshotFrame | CursorExpiredFrame;

export function useSessionStream({
  api,
  queryClient,
  sessionId,
}: {
  api: CopilotApi;
  queryClient: QueryClient;
  sessionId: string;
}) {
  const [state, setState] = useState<StreamState>("connecting");
  const [notice, setNotice] = useState("");
  const [output, setOutput] = useState("");
  const cursor = useRef("");
  const seenSequences = useRef(new Set<number>());
  const outputMessage = useRef<string | null>(null);

  useEffect(() => {
    if (!sessionId || typeof WebSocket === "undefined") return;

    let disposed = false;
    let socket: WebSocket | undefined;
    let retryTimer: number | undefined;
    let attempts = 0;

    const scheduleReconnect = () => {
      attempts += 1;
      retryTimer = window.setTimeout(
        connect,
        Math.min(10_000, 500 * 2 ** Math.min(attempts, 5)),
      );
    };
    const applySnapshot = (snapshot: SessionEventSnapshot) => {
      cursor.current = snapshot.cursor;
      seenSequences.current.clear();
      outputMessage.current = null;
      setOutput("");
      queryClient.setQueryData(["session", sessionId], snapshot.session);
      queryClient.setQueryData(["session-runs", sessionId], {
        pages: [
          {
            items: snapshot.runs,
            page_info: { has_more: false, next_cursor: null },
          },
        ],
        pageParams: [""],
      });
      queryClient.setQueryData(["session-approvals", sessionId], {
        pages: [
          {
            items: snapshot.approvals,
            page_info: { has_more: false, next_cursor: null },
          },
        ],
        pageParams: [""],
      });
    };
    const connect = async () => {
      setState(attempts ? "reconnecting" : "connecting");
      try {
        const ticket = await api.createSessionEventsTicket(
          sessionId,
          cursor.current || undefined,
        );
        if (disposed) return;

        const streamURL = new URL(
          ticket.websocket_url,
          window.location.origin,
        );
        streamURL.searchParams.set("ticket", ticket.ticket);
        if (cursor.current) {
          streamURL.searchParams.set("after", cursor.current);
        }
        socket = new WebSocket(streamURL.toString());
        socket.onopen = () => {
          attempts = 0;
          setState("connected");
        };
        socket.onmessage = ({ data }) => {
          let frame: StreamFrame;
          try {
            frame = JSON.parse(data as string) as StreamFrame;
          } catch {
            return;
          }

          if (isCursorExpired(frame)) {
            applySnapshot(frame.snapshot);
            setNotice(
              "Earlier live history expired; the current session projection was refreshed.",
            );
            return;
          }
          if (isSnapshot(frame)) {
            applySnapshot(frame);
            return;
          }
          if (
            !isEvent(frame) ||
            seenSequences.current.has(frame.session_sequence)
          ) {
            return;
          }

          seenSequences.current.add(frame.session_sequence);
          cursor.current = frame.cursor;
          const event = frame.event;
          switch (event.type) {
            case "content_segment":
              if (outputMessage.current !== event.message_id) {
                outputMessage.current = event.message_id;
                setOutput("");
              }
              setOutput((value) => value + event.text);
              break;
            case "message_committed":
              if (outputMessage.current === event.message.id) {
                outputMessage.current = null;
                setOutput("");
              }
              queryClient.setQueryData(
                ["session", sessionId],
                (current: Session | undefined) =>
                  current
                    ? {
                        ...current,
                        messages: [...current.messages, event.message],
                      }
                    : current,
              );
              break;
            case "run_state_changed":
              void queryClient.invalidateQueries({
                queryKey: ["session-runs", sessionId],
              });
              void queryClient.invalidateQueries({
                queryKey: ["session", sessionId],
              });
              break;
            case "session_changed":
              queryClient.setQueryData(
                ["session", sessionId],
                (current: Session | undefined) =>
                  current
                    ? {
                        ...current,
                        state: event.state,
                        mode: event.mode,
                        conversation_revision: event.conversation_revision,
                        queued_run_count: event.queued_run_count,
                        ...(event.members ? { members: event.members } : {}),
                      }
                    : current,
              );
              break;
            case "approval_changed":
              void queryClient.invalidateQueries({
                queryKey: ["session-approvals", sessionId],
              });
              break;
            case "artifact_changed":
              void queryClient.invalidateQueries({
                queryKey: ["session", sessionId],
              });
              break;
          }
        };
        socket.onerror = () => socket?.close();
        socket.onclose = () => {
          if (!disposed) scheduleReconnect();
        };
      } catch {
        scheduleReconnect();
      }
    };

    void connect();
    return () => {
      disposed = true;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
      socket?.close();
      setState("closed");
    };
  }, [api, queryClient, sessionId]);

  return { state, notice, output };
}

function isEvent(frame: StreamFrame): frame is SessionEventFrame {
  return "session_sequence" in frame && "event" in frame;
}

function isSnapshot(frame: StreamFrame): frame is SessionEventSnapshot {
  return (
    "schema_version" in frame &&
    frame.schema_version === "gantry.copilot.snapshot/v1"
  );
}

function isCursorExpired(frame: StreamFrame): frame is CursorExpiredFrame {
  return "type" in frame && frame.type === "cursor_expired";
}
