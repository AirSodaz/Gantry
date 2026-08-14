use prost_types::{Struct, Value, value::Kind};
use protocol::gantry::runner::v1::{
    ApprovalDecisionType, ApprovalResolution, AssignRun, CancelRun, RunAccepted, RunEvent,
    RunEventBatch, RunFinished, RunTerminalStatus, RunnerMessage, runner_message,
};
use serde::Deserialize;

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
    mode: DemoMode,
    waiting_for_approval: bool,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum DemoMode {
    Complete,
    AwaitCancel,
    AwaitApproval,
}

#[derive(Deserialize)]
struct DemoManifest {
    kind: String,
    mode: String,
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
        let Some(mode) = parse_demo_mode(&assignment.manifest) else {
            return Vec::new();
        };
        self.active = Some(ActiveRun {
            run_id: assignment.run_id.clone(),
            lease_epoch: assignment.lease_epoch,
            manifest_digest: assignment.manifest_digest.clone(),
            next_event_sequence: 1,
            step: 0,
            mode,
            waiting_for_approval: false,
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

    // Each tick emits one deterministic progress event. Approval-mode runs
    // pause after proposing one write action until the control plane resolves it.
    pub fn tick(&mut self) -> Vec<RunnerMessage> {
        let Some(run) = self.active.as_mut() else {
            return Vec::new();
        };
        if run.waiting_for_approval {
            return Vec::new();
        }
        if run.mode == DemoMode::AwaitApproval && run.step == 0 {
            run.next_event_sequence += 1;
            run.step += 1;
            let run_id = run.run_id.clone();
            let lease_epoch = run.lease_epoch;
            run.waiting_for_approval = true;
            return vec![
                self.message(runner_message::Payload::EventBatch(RunEventBatch {
                    run_id,
                    lease_epoch,
                    events: vec![RunEvent {
                        client_sequence: 1,
                        event_type: "action.proposed".into(),
                        occurred_at: None,
                        payload: Some(action_payload()),
                    }],
                })),
            ];
        }
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
        let finish = run.mode == DemoMode::Complete && run.step >= 2;
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

    pub fn resolve_approval(&mut self, resolution: &ApprovalResolution) -> Vec<RunnerMessage> {
        let (run_id, lease_epoch, client_sequence) = {
            let Some(run) = self.active.as_mut() else {
                return Vec::new();
            };
            if !run.waiting_for_approval || resolution.run_id != run.run_id {
                return Vec::new();
            }
            run.waiting_for_approval = false;
            (run.run_id.clone(), run.lease_epoch, run.next_event_sequence)
        };
        let approved = resolution.decision == ApprovalDecisionType::Approved as i32;
        let event_type = if approved {
            "action.approved"
        } else {
            "action.rejected"
        };
        let terminal = if approved {
            RunTerminalStatus::Completed
        } else {
            RunTerminalStatus::Failed
        };
        let reason = if resolution.reason.is_empty() {
            "approval resolved"
        } else {
            &resolution.reason
        };
        vec![
            self.message(runner_message::Payload::EventBatch(RunEventBatch {
                run_id: run_id.clone(),
                lease_epoch,
                events: vec![RunEvent {
                    client_sequence,
                    event_type: event_type.into(),
                    occurred_at: None,
                    payload: None,
                }],
            })),
            self.message(runner_message::Payload::RunFinished(RunFinished {
                run_id,
                lease_epoch,
                status: terminal as i32,
                reason: reason.into(),
            })),
        ]
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

fn parse_demo_mode(manifest: &[u8]) -> Option<DemoMode> {
    let manifest: DemoManifest = serde_json::from_slice(manifest).ok()?;
    if manifest.kind != "gantry.phase0.demo/v1" {
        return None;
    }
    match manifest.mode.as_str() {
        "complete" => Some(DemoMode::Complete),
        "await_cancel" => Some(DemoMode::AwaitCancel),
        "await_approval" => Some(DemoMode::AwaitApproval),
        _ => None,
    }
}

fn action_payload() -> Struct {
    let mut fields = std::collections::BTreeMap::new();
    for (key, value) in [
        ("tool_name", "demo.crm"),
        ("operation", "update_record"),
        ("target", "demo://record/1"),
        ("effect", "write"),
        ("credential_ref", "demo-write"),
        ("credential_mode", "platform"),
    ] {
        fields.insert(
            key.into(),
            Value {
                kind: Some(Kind::StringValue(value.into())),
            },
        );
    }
    Struct { fields }
}

#[cfg(test)]
mod tests {
    use super::*;
    use protocol::gantry::runner::v1::runner_message;

    fn assignment() -> AssignRun {
        assignment_with_mode("complete")
    }

    fn assignment_with_mode(mode: &str) -> AssignRun {
        AssignRun {
            run_id: "run-1".into(),
            lease_epoch: 7,
            manifest: format!(r#"{{"kind":"gantry.phase0.demo/v1","mode":"{mode}"}}"#).into_bytes(),
            manifest_digest: "sha256:demo".into(),
            assignment_expiry: None,
        }
    }

    #[test]
    fn approval_mode_proposes_once_and_waits() {
        let mut executor = DemoExecutor::new("runner-1");
        executor.assign(&assignment_with_mode("await_approval"));
        let messages = executor.tick();
        assert_eq!(messages.len(), 1);
        assert!(executor.tick().is_empty());
    }

    #[test]
    fn approval_resolution_finishes_with_decision() {
        let mut executor = DemoExecutor::new("runner-1");
        executor.assign(&assignment_with_mode("await_approval"));
        executor.tick();
        let messages = executor.resolve_approval(&ApprovalResolution {
            run_id: "run-1".into(),
            approval_request_id: "apr-1".into(),
            decision: ApprovalDecisionType::Approved as i32,
            reason: String::new(),
        });
        assert_eq!(messages.len(), 2);
        let Some(runner_message::Payload::RunFinished(finished)) = &messages[1].payload else {
            panic!("expected finish")
        };
        assert_eq!(finished.status, RunTerminalStatus::Completed as i32);
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

    #[test]
    fn await_cancel_mode_does_not_complete_on_ticks() {
        let mut executor = DemoExecutor::new("runner-1");
        executor.assign(&assignment_with_mode("await_cancel"));
        let first_tick = executor.tick();
        let second_tick = executor.tick();
        let third_tick = executor.tick();
        assert_eq!(first_tick.len(), 1);
        assert_eq!(second_tick.len(), 1);
        assert_eq!(third_tick.len(), 1);
        let Some(runner_message::Payload::EventBatch(event)) = &third_tick[0].payload else {
            panic!("expected event batch")
        };
        assert_eq!(event.events[0].client_sequence, 3);
    }

    #[test]
    fn malformed_demo_manifest_is_not_accepted() {
        let mut executor = DemoExecutor::new("runner-1");
        let mut invalid = assignment();
        invalid.manifest = b"not-json".to_vec();
        assert!(executor.assign(&invalid).is_empty());
        assert!(executor.tick().is_empty());
    }
}
