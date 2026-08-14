use protocol::gantry::runner::v1::{
    AssignRun, CancelRun, RunAccepted, RunEvent, RunEventBatch, RunFinished, RunTerminalStatus,
    RunnerMessage, runner_message,
};

const PROTOCOL_VERSION: u32 = 1;

pub struct DemoExecutor {
    runner_id: String,
    session_id: String,
    next_message_id: u64,
    active: Option<ActiveRun>,
}

struct ActiveRun {
    run_id: String,
    lease_epoch: u64,
    manifest_digest: String,
    next_event_sequence: u64,
    step: u8,
}

impl DemoExecutor {
    pub fn new(runner_id: impl Into<String>) -> Self {
        let runner_id = runner_id.into();
        Self {
            session_id: format!("{runner_id}-session"),
            runner_id,
            next_message_id: 1,
            active: None,
        }
    }

    pub fn register(&mut self) -> RunnerMessage {
        self.message(runner_message::Payload::Register(
            protocol::gantry::runner::v1::RegisterRunner {
                runner_version: env!("CARGO_PKG_VERSION").into(),
                capabilities: vec!["phase0.session".into()],
                organization_id: "dev".into(),
                resource_limits: None,
            },
        ))
    }

    pub fn heartbeat(&mut self) -> RunnerMessage {
        self.message(runner_message::Payload::Heartbeat(
            protocol::gantry::runner::v1::Heartbeat {
                timestamp: None,
                run_id: self
                    .active
                    .as_ref()
                    .map_or_else(String::new, |run| run.run_id.clone()),
                lease_epoch: self.active.as_ref().map_or(0, |run| run.lease_epoch),
                status: if self.active.is_some() { 2 } else { 1 },
            },
        ))
    }

    pub fn assign(&mut self, assignment: &AssignRun) -> Vec<RunnerMessage> {
        if self.active.is_some()
            || assignment.run_id.is_empty()
            || assignment.lease_epoch == 0
            || assignment.manifest.is_empty()
            || assignment.manifest_digest.is_empty()
        {
            return Vec::new();
        }
        self.active = Some(ActiveRun {
            run_id: assignment.run_id.clone(),
            lease_epoch: assignment.lease_epoch,
            manifest_digest: assignment.manifest_digest.clone(),
            next_event_sequence: 1,
            step: 0,
        });
        let run = self.active.as_ref().expect("assigned run is present");
        vec![
            self.message(runner_message::Payload::RunAccepted(RunAccepted {
                run_id: run.run_id.clone(),
                lease_epoch: run.lease_epoch,
                manifest_digest: run.manifest_digest.clone(),
            })),
        ]
    }

    // Each tick emits one deterministic progress event. The second tick emits
    // completion, giving cancellation one observable control-plane window.
    pub fn tick(&mut self) -> Vec<RunnerMessage> {
        let Some(run) = self.active.as_mut() else {
            return Vec::new();
        };
        let event = RunEvent {
            client_sequence: run.next_event_sequence,
            event_type: format!("demo.step.{}", run.step + 1),
            occurred_at: None,
            payload: None,
        };
        run.next_event_sequence += 1;
        run.step += 1;
        let run_id = run.run_id.clone();
        let lease_epoch = run.lease_epoch;
        let finish = run.step >= 2;
        let mut messages = vec![
            self.message(runner_message::Payload::EventBatch(RunEventBatch {
                run_id: run_id.clone(),
                lease_epoch,
                events: vec![event],
            })),
        ];
        if finish {
            self.active = None;
            messages.push(
                self.message(runner_message::Payload::RunFinished(RunFinished {
                    run_id,
                    lease_epoch,
                    status: RunTerminalStatus::Completed as i32,
                    reason: "demo complete".into(),
                })),
            );
        }
        messages
    }

    pub fn cancel(&mut self, cancel: &CancelRun) -> Vec<RunnerMessage> {
        let Some(run) = self.active.as_ref() else {
            return Vec::new();
        };
        if cancel.run_id != run.run_id || cancel.lease_epoch != run.lease_epoch {
            return Vec::new();
        }
        let run_id = run.run_id.clone();
        let lease_epoch = run.lease_epoch;
        self.active = None;
        vec![
            self.message(runner_message::Payload::RunFinished(RunFinished {
                run_id,
                lease_epoch,
                status: RunTerminalStatus::Canceled as i32,
                reason: cancel.reason.clone(),
            })),
        ]
    }

    fn message(&mut self, payload: runner_message::Payload) -> RunnerMessage {
        let message = RunnerMessage {
            runner_id: self.runner_id.clone(),
            session_id: self.session_id.clone(),
            message_id: self.next_message_id,
            protocol_version: PROTOCOL_VERSION,
            payload: Some(payload),
        };
        self.next_message_id += 1;
        message
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use protocol::gantry::runner::v1::runner_message;

    fn assignment() -> AssignRun {
        AssignRun {
            run_id: "run-1".into(),
            lease_epoch: 7,
            manifest: b"demo".to_vec(),
            manifest_digest: "sha256:demo".into(),
            assignment_expiry: None,
        }
    }

    #[test]
    fn assignment_emits_ordered_events_and_completion() {
        let mut executor = DemoExecutor::new("runner-1");
        let registration = executor.register();
        let accepted = executor.assign(&assignment());
        let first_tick = executor.tick();
        let second_tick = executor.tick();
        assert_eq!(registration.message_id, 1);
        assert_eq!(accepted[0].message_id, 2);
        let Some(runner_message::Payload::EventBatch(first_event)) = &first_tick[0].payload else {
            panic!("expected event batch")
        };
        let Some(runner_message::Payload::EventBatch(second_event)) = &second_tick[0].payload
        else {
            panic!("expected event batch")
        };
        let Some(runner_message::Payload::RunFinished(completion)) = &second_tick[1].payload else {
            panic!("expected completion")
        };
        assert_eq!(first_event.events[0].client_sequence, 1);
        assert_eq!(second_event.events[0].client_sequence, 2);
        assert_eq!(completion.status, RunTerminalStatus::Completed as i32);
    }

    #[test]
    fn matching_cancellation_finishes_canceled() {
        let mut executor = DemoExecutor::new("runner-1");
        executor.assign(&assignment());
        let messages = executor.cancel(&CancelRun {
            run_id: "run-1".into(),
            lease_epoch: 7,
            reason: "requested".into(),
            force: false,
        });
        let Some(runner_message::Payload::RunFinished(completion)) = &messages[0].payload else {
            panic!("expected cancellation")
        };
        assert_eq!(completion.status, RunTerminalStatus::Canceled as i32);
        assert!(executor.tick().is_empty());
    }

    #[test]
    fn non_matching_cancellation_is_ignored() {
        let mut executor = DemoExecutor::new("runner-1");
        executor.assign(&assignment());
        assert!(
            executor
                .cancel(&CancelRun {
                    run_id: "other".into(),
                    lease_epoch: 7,
                    reason: String::new(),
                    force: false
                })
                .is_empty()
        );
        assert!(
            executor
                .cancel(&CancelRun {
                    run_id: "run-1".into(),
                    lease_epoch: 8,
                    reason: String::new(),
                    force: false
                })
                .is_empty()
        );
        assert!(!executor.tick().is_empty());
    }
}
