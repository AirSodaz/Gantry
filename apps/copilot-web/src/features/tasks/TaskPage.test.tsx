import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TaskPage } from "./TaskPage";

const mocked = vi.hoisted(() => ({
  api: {
    getTask: vi.fn(),
    createEventsTicket: vi.fn(),
    requestArtifactDownload: vi.fn(),
    cancelRun: vi.fn(),
    retryTask: vi.fn(),
    appendTaskMessage: vi.fn(),
    listTaskRuns: vi.fn(),
    listApprovals: vi.fn(),
  },
}));
vi.mock("../../api/ApiProvider", () => ({ useCopilotApi: () => mocked.api }));

const baseTask = {
  id: "tsk_1",
  agent_id: "agt_1",
  agent_display_name: "Lifecycle Demo",
  created_at: "2026-08-14T08:00:00Z",
  current_run: { id: "run_1", status: "running" },
};

function renderTask() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/tasks/tsk_1"]}>
        <Routes>
          <Route path="/tasks/:taskId" element={<TaskPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TaskPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    MockWebSocket.instances = [];
  });
  afterEach(() => vi.unstubAllGlobals());

  it("shows cancellation for an active run and sends the fenced run identity", async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: "running" });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      expires_at: "2026-08-14T08:01:00Z",
    });
    mocked.api.cancelRun.mockResolvedValue({
      id: "run_1",
      status: "canceling",
    });
    const user = userEvent.setup();
    renderTask();

    const cancel = await screen.findByRole("button", { name: /Cancel run/ });
    await user.click(cancel);

    await waitFor(() =>
      expect(mocked.api.cancelRun).toHaveBeenCalledWith(
        "tsk_1",
        "run_1",
        expect.any(String),
      ),
    );
  });

  it("polls while the task remains active", async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: "running" });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      expires_at: "2026-08-14T08:01:00Z",
    });
    renderTask();

    await screen.findByText("Task tsk_1");
    await waitFor(
      () => expect(mocked.api.getTask.mock.calls.length).toBeGreaterThan(1),
      { timeout: 2_500 },
    );
  });

  it("shows retry only for failed tasks and starts a new task run", async () => {
    mocked.api.getTask.mockResolvedValue({
      ...baseTask,
      status: "failed",
      current_run: {
        id: "run_1",
        status: "failed",
        status_reason: "Runner disconnected.",
      },
    });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      expires_at: "2026-08-14T08:01:00Z",
    });
    mocked.api.getTask.mockResolvedValue({
      ...baseTask,
      status: "failed",
      conversation_etag: '"1"',
      current_run: {
        id: "run_1",
        status: "failed",
        status_reason: "Runner disconnected.",
      },
    });
    mocked.api.retryTask.mockResolvedValue({
      ...baseTask,
      status: "queued",
      conversation_etag: '"2"',
      current_run: { id: "run_2", status: "assigned" },
    });
    const user = userEvent.setup();
    renderTask();

    expect(await screen.findByText("Runner disconnected.")).toBeInTheDocument();
    const retry = screen.getByRole("button", { name: /Retry task/ });
    await user.click(retry);

    await waitFor(() =>
      expect(mocked.api.retryTask).toHaveBeenCalledWith(
        "tsk_1",
        expect.any(String),
        '"1"',
        "original_revision",
      ),
    );
  });

  it("accumulates event output and reconnects with the rendered cursor", async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: "running" });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      websocket_url:
        "wss://stream.example.test/api/copilot/v1/tasks/tsk_1/events",
      expires_at: "2026-08-14T08:01:00Z",
    });
    mocked.api.listTaskRuns.mockResolvedValue({ items: [] });
    vi.stubGlobal("WebSocket", MockWebSocket);
    renderTask();

    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    expect(MockWebSocket.instances[0].url).toContain("stream.example.test");
    MockWebSocket.instances[0].emit({
      schema_version: "gantry.copilot.event/v1",
      task_id: "tsk_1",
      run_id: "run_1",
      task_sequence: 1,
      run_sequence: 1,
      cursor: "cur_1",
      event: {
        type: "content_segment",
        message_id: "msg_1",
        segment_index: 0,
        text: "Hello",
      },
    });
    expect(await screen.findByText("Hello")).toBeInTheDocument();

    MockWebSocket.instances[0].close();
    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(2), {
      timeout: 1_500,
    });
    expect(MockWebSocket.instances[1].url).toContain("after=cur_1");
  });

  it("replaces provisional output when its message is committed", async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: "running" });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      websocket_url:
        "wss://stream.example.test/api/copilot/v1/tasks/tsk_1/events",
      expires_at: "2026-08-14T08:01:00Z",
    });
    vi.stubGlobal("WebSocket", MockWebSocket);
    renderTask();

    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    MockWebSocket.instances[0].emit({
      schema_version: "gantry.copilot.event/v1",
      task_id: "tsk_1",
      run_id: "run_1",
      task_sequence: 1,
      run_sequence: 1,
      cursor: "cur_1",
      event: {
        type: "content_segment",
        message_id: "msg_1",
        segment_index: 0,
        text: "Provisional reply",
      },
    });
    expect(await screen.findByText("Provisional reply")).toBeInTheDocument();
    MockWebSocket.instances[0].emit({
      schema_version: "gantry.copilot.event/v1",
      task_id: "tsk_1",
      run_id: "run_1",
      task_sequence: 2,
      run_sequence: 2,
      cursor: "cur_2",
      event: {
        type: "message_committed",
        message: {
          id: "msg_1",
          task_sequence: 2,
          role: "agent",
          parts: [{ type: "text", text: "Provisional reply" }],
          created_at: "2026-08-14T08:00:00Z",
        },
      },
    });

    await waitFor(() =>
      expect(
        screen.getByText("Live output").closest(".run-output"),
      ).not.toHaveTextContent("Provisional reply"),
    );
  });

  it("loads additional compact run attempts from the next cursor", async () => {
    mocked.api.getTask.mockResolvedValue({
      ...baseTask,
      status: "completed",
      current_run: { id: "run_2", status: "completed" },
    });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      expires_at: "2026-08-14T08:01:00Z",
    });
    mocked.api.listTaskRuns
      .mockResolvedValueOnce({
        items: [
          {
            id: "run_2",
            attempt_number: 2,
            status: "completed",
            created_at: "2026-08-14T08:02:00Z",
          },
        ],
        page_info: { has_more: true, next_cursor: "run_cursor_1" },
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: "run_1",
            attempt_number: 1,
            status: "failed",
            created_at: "2026-08-14T08:00:00Z",
          },
        ],
        page_info: { has_more: false },
      });
    const user = userEvent.setup();
    renderTask();

    expect(await screen.findByText("Attempt 2")).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Load more run attempts" }),
    );

    await waitFor(() =>
      expect(mocked.api.listTaskRuns).toHaveBeenLastCalledWith(
        "tsk_1",
        "run_cursor_1",
      ),
    );
    expect(await screen.findByText("Attempt 1")).toBeInTheDocument();
  });

  it("replaces the task projection and cursor from a stream snapshot", async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: "running" });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      websocket_url:
        "wss://stream.example.test/api/copilot/v1/tasks/tsk_1/events",
      expires_at: "2026-08-14T08:01:00Z",
    });
    mocked.api.listTaskRuns.mockResolvedValue({ items: [] });
    mocked.api.listApprovals.mockImplementation(() => new Promise(() => {}));
    vi.stubGlobal("WebSocket", MockWebSocket);
    renderTask();

    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    MockWebSocket.instances[0].emit({
      type: "snapshot",
      schema_version: "gantry.copilot.snapshot/v1",
      cursor: "cur_snapshot",
      task: {
        ...baseTask,
        agent_display_name: "Snapshot Agent",
        status: "awaiting_approval",
        current_run: { id: "run_1", status: "awaiting_approval" },
      },
      runs: [
        {
          id: "run_1",
          attempt_number: 1,
          status: "awaiting_approval",
          created_at: "2026-08-14T08:00:00Z",
        },
      ],
      approvals: [{ id: "apr_1", run_id: "run_1", status: "pending" }],
    });
    expect(
      await screen.findByRole("heading", { name: "Snapshot Agent" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Open approval" }),
    ).toBeInTheDocument();

    MockWebSocket.instances[0].close();
    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(2), {
      timeout: 1_500,
    });
    expect(MockWebSocket.instances[1].url).toContain("after=cur_snapshot");
  });

  it("explains when expired stream history is replaced by a snapshot", async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: "running" });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      websocket_url:
        "wss://stream.example.test/api/copilot/v1/tasks/tsk_1/events",
      expires_at: "2026-08-14T08:01:00Z",
    });
    vi.stubGlobal("WebSocket", MockWebSocket);
    renderTask();

    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    MockWebSocket.instances[0].emit({
      type: "cursor_expired",
      snapshot: {
        type: "snapshot",
        schema_version: "gantry.copilot.snapshot/v1",
        cursor: "cur_fresh",
        task: {
          ...baseTask,
          status: "completed",
          current_run: { id: "run_1", status: "completed" },
        },
        runs: [],
        approvals: [],
      },
    });

    expect(
      await screen.findByText(
        "Earlier live history expired, so the current task state was refreshed.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a retry action when an artifact download fails", async () => {
    mocked.api.getTask.mockResolvedValue({
      ...baseTask,
      status: "completed",
      current_run: { id: "run_1", status: "completed" },
      artifacts: [
        {
          id: "art_1",
          filename: "result.txt",
          media_type: "text/plain",
          size_bytes: 3,
          state: "available",
          scan_status: "passed",
        },
      ],
    });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      expires_at: "2026-08-14T08:01:00Z",
    });
    mocked.api.requestArtifactDownload
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce({
        download_url: "http://example.test/result.txt",
      });
    const user = userEvent.setup();
    renderTask();

    await user.click(await screen.findByRole("button", { name: "Download" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Download failed",
    );
    expect(
      screen.getByRole("button", { name: "Retry download" }),
    ).toBeInTheDocument();
  });

  it("continues an approval-rejected task through a new idempotent message", async () => {
    mocked.api.getTask.mockResolvedValue({
      ...baseTask,
      status: "awaiting_requester_input",
      conversation_etag: '"4"',
      current_run: { id: "run_1", status: "failed" },
      messages: [
        {
          id: "msg_1",
          role: "requester",
          content: "Update the record",
          created_at: "2026-08-16T08:00:00Z",
        },
      ],
    });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      expires_at: "2026-08-14T08:01:00Z",
    });
    mocked.api.appendTaskMessage.mockResolvedValue({
      ...baseTask,
      status: "queued",
      current_run: { id: "run_2", status: "queued" },
    });
    const user = userEvent.setup();
    renderTask();

    await user.type(
      await screen.findByLabelText("Continue this task"),
      "Use a different target",
    );
    await user.click(screen.getByRole("button", { name: "Continue" }));

    await waitFor(() =>
      expect(mocked.api.appendTaskMessage).toHaveBeenCalledWith(
        "tsk_1",
        { message: "Use a different target" },
        expect.any(String),
        '"4"',
      ),
    );
  });

  it("renders every documented task message part", async () => {
    mocked.api.getTask.mockResolvedValue({
      ...baseTask,
      status: "completed",
      messages: [
        {
          id: "msg_text",
          task_sequence: 1,
          role: "agent",
          parts: [{ type: "text", text: "Agent response" }],
          created_at: "2026-08-16T08:00:00Z",
        },
        {
          id: "msg_artifact",
          task_sequence: 2,
          role: "system_summary",
          parts: [
            { type: "artifact", artifact_id: "art_1", label: "report.csv" },
          ],
          created_at: "2026-08-16T08:00:01Z",
        },
        {
          id: "msg_action",
          task_sequence: 3,
          role: "system_summary",
          parts: [
            {
              type: "action_summary",
              action_id: "act_1",
              summary: "Update customer record",
              state: "succeeded",
            },
          ],
          created_at: "2026-08-16T08:00:02Z",
        },
        {
          id: "msg_status",
          task_sequence: 4,
          role: "system_summary",
          parts: [
            {
              type: "status",
              code: "run.completed",
              message: "Run completed.",
            },
          ],
          created_at: "2026-08-16T08:00:03Z",
        },
      ],
    });
    mocked.api.createEventsTicket.mockResolvedValue({
      ticket: "evt.test",
      task_id: "tsk_1",
      expires_at: "2026-08-14T08:01:00Z",
    });
    renderTask();

    expect(await screen.findByText("Agent response")).toBeInTheDocument();
    expect(screen.getByText("report.csv")).toBeInTheDocument();
    expect(screen.getByText("Update customer record")).toBeInTheDocument();
    expect(screen.getByText("Run completed.")).toBeInTheDocument();
  });
});

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(readonly url: string) {
    MockWebSocket.instances.push(this);
    queueMicrotask(() => this.onopen?.(new Event("open")));
  }

  close() {
    this.onclose?.(new Event("close"));
  }
  emit(frame: unknown) {
    this.onmessage?.(
      new MessageEvent("message", { data: JSON.stringify(frame) }),
    );
  }
}
