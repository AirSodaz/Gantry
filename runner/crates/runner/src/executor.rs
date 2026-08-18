use policy_enforcement::check_command;
use prost_types::{Struct, Value, value::Kind};
use protocol::{
    gantry::runner::v1::{
        AcknowledgeEvents, ApprovalDecisionType, ApprovalResolution, ArtifactDeclaration,
        ArtifactUploadCompleted, ArtifactUploadGrant, AssignRun, CancelRun, RunAccepted, RunEvent,
        RunEventBatch, RunFinished, RunTerminalStatus, RunnerMessage, runner_message,
    },
    types::{RUNNER_MANIFEST_KIND, RunManifest},
};
use serde_json::{Value as JsonValue, json};
use std::collections::HashMap;
use std::time::Duration;
use tokio_util::sync::CancellationToken;

use crate::{
    checkpoint::CheckpointStore,
    context::ContextState,
    model::{ModelEvent, from_config},
    rules::RuleEngine,
    tools::WorkspaceTools,
};

const PROTOCOL_VERSION: u32 = 1;

pub struct AgentExecutor {
    runner_id: String,
    session_id: String,
    next_message_id: u64,
    active: Option<ActiveRun>,
    artifact_sources: HashMap<String, String>,
}

struct ActiveRun {
    run_id: String,
    lease_epoch: u64,
    manifest: RunManifest,
    manifest_digest: String,
    next_event_sequence: u64,
    turn: u32,
    output_bytes: usize,
    context: ContextState,
    rules: RuleEngine,
    tools: WorkspaceTools,
    model: Box<dyn crate::model::ModelClient>,
    waiting_for_approval: bool,
    execution_requested: bool,
    execution_granted: bool,
    cancel: CancellationToken,
    checkpoint: Option<CheckpointStore>,
    suspended: bool,
}

impl AgentExecutor {
    pub fn new(runner_id: impl Into<String>) -> Self {
        let runner_id = runner_id.into();
        Self {
            session_id: format!("{runner_id}-session"),
            runner_id,
            next_message_id: 1,
            active: None,
            artifact_sources: HashMap::new(),
        }
    }

    pub fn register(&mut self) -> RunnerMessage {
        self.message(runner_message::Payload::Register(
            protocol::gantry::runner::v1::RegisterRunner {
                runner_version: env!("CARGO_PKG_VERSION").into(),
                capabilities: vec![
                    "agent.v1".into(),
                    "native-tools.v1".into(),
                    "stream-rules.v1".into(),
                ],
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
        if self.active.is_some() || assignment.run_id.is_empty() || assignment.lease_epoch == 0 {
            return Vec::new();
        }
        if assignment.manifest.is_empty() {
            return self.reject_assignment(assignment);
        }
        let Ok(manifest) = serde_json::from_slice::<RunManifest>(&assignment.manifest) else {
            return self.reject_assignment(assignment);
        };
        if manifest.kind != RUNNER_MANIFEST_KIND || manifest.validate().is_err() {
            return self.reject_assignment(assignment);
        }
        let digest = RunManifest::digest_bytes(&assignment.manifest);
        if !assignment.manifest_digest.is_empty() && digest != assignment.manifest_digest {
            return self.reject_assignment(assignment);
        }
        let Ok(tools) =
            WorkspaceTools::new(&manifest.workspace_root, manifest.limits.max_output_bytes)
        else {
            return self.reject_assignment(assignment);
        };
        let Ok(model) = from_config(&manifest.model) else {
            return self.reject_assignment(assignment);
        };
        let mut context = ContextState {
            branch: assignment.run_id.clone(),
            ..Default::default()
        };
        let checkpoint = if manifest.checkpoint.enabled {
            let Some(path) = manifest.checkpoint.path.clone() else {
                return self.reject_assignment(assignment);
            };
            if !checkpoint_path_allowed(&manifest.workspace_root, &path) {
                return self.reject_assignment(assignment);
            }
            let checkpoint_path = if std::path::Path::new(&path).is_absolute() {
                path.clone()
            } else {
                std::path::Path::new(&manifest.workspace_root)
                    .join(path)
                    .to_string_lossy()
                    .into_owned()
            };
            let Ok(store) = CheckpointStore::from_env(checkpoint_path) else {
                return self.reject_assignment(assignment);
            };
            match store.load(&assignment.run_id, assignment.lease_epoch, &manifest) {
                Ok(Some(restored)) => context = restored,
                Ok(None) => {}
                Err(_) => return self.reject_assignment(assignment),
            }
            Some(store)
        } else {
            None
        };
        let user_rules = std::env::var_os("GANTRY_USER_RULES_DIR");
        let rules = RuleEngine::discover(
            std::path::Path::new(&manifest.workspace_root),
            user_rules.as_deref().map(std::path::Path::new),
        )
        .with_manifest(manifest.rules.clone());
        let artifact_declarations = artifact_declarations(
            &manifest,
            &assignment.run_id,
            assignment.lease_epoch,
            &mut self.artifact_sources,
        );
        self.active = Some(ActiveRun {
            run_id: assignment.run_id.clone(),
            lease_epoch: assignment.lease_epoch,
            manifest,
            manifest_digest: digest.clone(),
            next_event_sequence: 1,
            turn: 0,
            output_bytes: 0,
            context,
            rules,
            tools,
            model,
            waiting_for_approval: false,
            execution_requested: false,
            execution_granted: false,
            cancel: CancellationToken::new(),
            checkpoint,
            suspended: false,
        });
        let run = self.active.as_ref().unwrap();
        let mut messages = vec![
            self.message(runner_message::Payload::RunAccepted(RunAccepted {
                run_id: run.run_id.clone(),
                lease_epoch: run.lease_epoch,
                manifest_digest: run.manifest_digest.clone(),
            })),
        ];
        messages.extend(artifact_declarations.into_iter().map(|declaration| {
            self.message(runner_message::Payload::ArtifactDeclaration(declaration))
        }));
        messages
    }

    fn reject_assignment(&mut self, assignment: &AssignRun) -> Vec<RunnerMessage> {
        vec![
            self.message(runner_message::Payload::RunFinished(RunFinished {
                run_id: assignment.run_id.clone(),
                lease_epoch: assignment.lease_epoch,
                status: RunTerminalStatus::Failed as i32,
                reason: "Runner could not prepare the assigned execution environment.".into(),
            })),
        ]
    }

    pub async fn upload_artifact(
        &mut self,
        grant: &ArtifactUploadGrant,
        http_address: &str,
    ) -> Option<RunnerMessage> {
        let source = self.artifact_sources.remove(&grant.artifact_id)?;
        if grant.run_id.is_empty() || grant.lease_epoch == 0 || grant.upload_token.is_empty() {
            return None;
        }
        let bytes = tokio::fs::read(source).await.ok()?;
        let url = format!("{}{}", http_address.trim_end_matches('/'), grant.upload_url);
        let response = reqwest::Client::new()
            .post(url)
            .header("X-Gantry-Artifact-Token", &grant.upload_token)
            .body(bytes.clone())
            .send()
            .await
            .ok()?;
        if !response.status().is_success() {
            return None;
        }
        let digest = protocol::types::RunManifest::digest_bytes(&bytes);
        Some(
            self.message(runner_message::Payload::ArtifactUploadCompleted(
                ArtifactUploadCompleted {
                    run_id: grant.run_id.clone(),
                    lease_epoch: grant.lease_epoch,
                    artifact_id: grant.artifact_id.clone(),
                    digest,
                    size_bytes: bytes.len() as u64,
                },
            )),
        )
    }

    pub async fn tick(&mut self) -> Vec<RunnerMessage> {
        let Some(mut run) = self.active.take() else {
            return Vec::new();
        };
        if run.waiting_for_approval {
            self.active = Some(run);
            return Vec::new();
        }
        if run.suspended {
            self.active = Some(run);
            return Vec::new();
        }
        run.turn += 1;
        if run.turn > run.manifest.limits.max_turns {
            let message =
                self.finish_message(&run, RunTerminalStatus::Failed, "max turns exceeded");
            return vec![message];
        }
        let mut outbound = Vec::new();
        if let Some(action) = run.context.pending_action.take() {
            let name = action
                .get("name")
                .and_then(JsonValue::as_str)
                .unwrap_or_default()
                .to_string();
            let id = action
                .get("id")
                .and_then(JsonValue::as_str)
                .unwrap_or_default()
                .to_string();
            let arguments = action.get("arguments").cloned().unwrap_or(JsonValue::Null);
            let permit_id = action
                .get("permit_id")
                .and_then(JsonValue::as_str)
                .unwrap_or_default()
                .to_string();
            if !permit_id.is_empty() && !run.execution_granted {
                if run.execution_requested {
                    run.context.pending_action = Some(action);
                    self.active = Some(run);
                    return Vec::new();
                }
                let action_id = action
                    .get("action_id")
                    .and_then(JsonValue::as_str)
                    .unwrap_or_default()
                    .to_string();
                if action_id.is_empty() {
                    let message = self.finish_message(
                        &run,
                        RunTerminalStatus::Failed,
                        "execution permit action binding missing",
                    );
                    return vec![message];
                }
                run.execution_requested = true;
                run.context.pending_action = Some(action);
                let request = self.event(
                    &mut run,
                    "action.execution_requested",
                    payload(&[
                        ("action_id", &action_id),
                        ("call_id", &id),
                        ("permit_id", &permit_id),
                    ]),
                );
                self.active = Some(run);
                return vec![request];
            }
            let result = self.dispatch_tool(&run, &name, arguments).await;
            let result_text = tool_result_content(&result);
            run.context.push(
                "tool",
                json!({
                    "type": "untrusted_context",
                    "source": "tool",
                    "content": result_text.clone(),
                }),
            );
            outbound.push(self.event(
                &mut run,
                if result.ok {
                    "tool.call.completed"
                } else {
                    "tool.call.failed"
                },
                payload(&[("tool", &name), ("call_id", &id), ("content", &result_text)]),
            ));
            self.active = Some(run);
            return outbound;
        }

        if run.context.messages.is_empty() {
            run.context.push(
                "system",
                JsonValue::String(format!(
                    "{}{}",
                    run.manifest.system_prompt,
                    run.rules.system_injection("", "")
                )),
            );
            run.context
                .push("user", JsonValue::String(run.manifest.user_input.clone()));
        }
        let mut prompt = Vec::new();
        if let Some(summary) = &run.context.summary {
            prompt.push(json!({
                "role": "system",
                "content": {
                    "type": "untrusted_context",
                    "source": "context.compaction",
                    "digest": run.context.summary_digest.as_deref().unwrap_or_default(),
                    "content": summary,
                },
            }));
        }
        prompt.extend(
            run.context
                .messages
                .iter()
                .map(|message| json!({"role": message.role, "content": message.content})),
        );
        let model_request = run.model.stream(&run.manifest, &prompt, run.cancel.clone());
        let events = match if run.manifest.limits.timeout_seconds == 0 {
            Ok(model_request.await)
        } else {
            match tokio::time::timeout(
                Duration::from_secs(run.manifest.limits.timeout_seconds),
                model_request,
            )
            .await
            {
                Ok(result) => Ok(result),
                Err(_) => Err(anyhow::anyhow!("model request timed out")),
            }
        } {
            Ok(Ok(events)) => events,
            Ok(Err(error)) => {
                let message =
                    self.finish_message(&run, RunTerminalStatus::Failed, &error.to_string());
                return vec![message];
            }
            Err(error) => {
                let message =
                    self.finish_message(&run, RunTerminalStatus::Failed, &error.to_string());
                return vec![message];
            }
        };
        let mut saw_tool_call = false;
        let mut waiting = false;
        for event in events {
            match event {
                ModelEvent::Wait => {
                    waiting = true;
                    outbound.push(self.event(
                        &mut run,
                        "agent.waiting",
                        payload(&[("reason", "awaiting cancellation")]),
                    ));
                }
                ModelEvent::TextDelta(text) => {
                    let text = bounded_output(&mut run, &text);
                    if text.is_empty() {
                        continue;
                    }
                    for matched in run.rules.inspect_stream("assistant", None, None, &text) {
                        outbound.push(self.event(
                            &mut run,
                            "security.untrusted_context",
                            payload(&[("rule", &matched.name), ("action", "isolated")]),
                        ));
                    }
                    run.context
                        .push("assistant", JsonValue::String(text.clone()));
                    outbound.push(self.event(
                        &mut run,
                        "model.delta",
                        payload(&[("stream_id", "model"), ("text", &text)]),
                    ));
                }
                ModelEvent::ThinkingDelta(text) => {
                    let text = bounded_output(&mut run, &text);
                    if text.is_empty() {
                        continue;
                    }
                    for matched in run.rules.inspect_stream("thinking", None, None, &text) {
                        outbound.push(self.event(
                            &mut run,
                            "security.untrusted_context",
                            payload(&[("rule", &matched.name), ("action", "isolated")]),
                        ));
                    }
                    outbound.push(self.event(
                        &mut run,
                        "model.thinking",
                        payload(&[("summary", &text)]),
                    ));
                }
                ModelEvent::ToolCallDelta {
                    id,
                    name,
                    arguments,
                } => {
                    saw_tool_call = true;
                    let Some(arguments) = serde_json::from_str::<JsonValue>(&arguments).ok() else {
                        continue;
                    };
                    if let Some(matches) = run
                        .rules
                        .inspect_stream(
                            "tool",
                            arguments.get("path").and_then(JsonValue::as_str),
                            Some(&name),
                            &arguments.to_string(),
                        )
                        .first()
                    {
                        outbound.push(self.event(
                            &mut run,
                            "security.untrusted_context",
                            payload(&[("rule", &matches.name), ("action", "isolated")]),
                        ));
                        if matches.interrupt {
                            continue;
                        }
                    }
                    if name == "shell" && !run.waiting_for_approval {
                        run.context.pending_action = Some(json!({
                            "id": id,
                            "name": name,
                            "arguments": arguments,
                        }));
                        run.execution_requested = false;
                        run.execution_granted = false;
                        run.waiting_for_approval = true;
                        outbound.push(self.event(
                            &mut run,
                            "action.proposed",
                            action_proposal_payload(&id, &name, "execute", "write", &arguments),
                        ));
                        continue;
                    }
                    let result = self.dispatch_tool(&run, &name, arguments.clone()).await;
                    let result_text = tool_result_content(&result);
                    run.context.push(
                        "tool",
                        json!({
                            "type": "untrusted_context",
                            "source": "tool",
                            "content": result_text.clone(),
                        }),
                    );
                    outbound.push(self.event(
                        &mut run,
                        if result.ok {
                            "tool.call.completed"
                        } else {
                            "tool.call.failed"
                        },
                        payload(&[("tool", &name), ("call_id", &id), ("content", &result_text)]),
                    ));
                }
                ModelEvent::Usage {
                    input_tokens,
                    output_tokens,
                } => outbound.push(self.event(
                    &mut run,
                    "model.usage",
                    payload(&[
                        ("input_tokens", &input_tokens.to_string()),
                        ("output_tokens", &output_tokens.to_string()),
                    ]),
                )),
                ModelEvent::Done => {}
            }
        }
        let mut context_limits = run.manifest.limits.clone();
        if context_limits.context_soft_limit == 0 {
            context_limits.context_soft_limit =
                run.manifest.model.max_context_tokens.saturating_mul(8) / 10;
        }
        let report = run.context.compact(&context_limits);
        if report.compacted {
            outbound.push(self.event(
                &mut run,
                "context.compacted",
                payload(&[("digest", report.digest.as_deref().unwrap_or_default())]),
            ));
        }
        if let Some(warning) = report.warning {
            outbound.push(self.event(
                &mut run,
                "context.compaction_warning",
                payload(&[("warning", &warning)]),
            ));
        }
        if let Some(store) = &run.checkpoint {
            let checkpoint_result = store.save(
                &run.run_id,
                run.lease_epoch,
                &run.manifest,
                run.context.clone(),
            );
            let event_type = if checkpoint_result.is_ok() {
                "run.checkpoint_created"
            } else {
                "checkpoint.failed"
            };
            outbound.push(self.event(
                &mut run,
                event_type,
                payload(&[("path", "encrypted-local")]),
            ));
        }
        if run.waiting_for_approval || saw_tool_call || waiting {
            self.active = Some(run);
            return outbound;
        }
        outbound.push(self.finish_message(&run, RunTerminalStatus::Completed, "agent completed"));
        outbound
    }

    pub fn resolve_approval(&mut self, resolution: &ApprovalResolution) -> Vec<RunnerMessage> {
        let Some(mut run) = self.active.take() else {
            return Vec::new();
        };
        if !run.waiting_for_approval
            || run.run_id != resolution.run_id
            || (resolution.lease_epoch != 0 && run.lease_epoch != resolution.lease_epoch)
        {
            self.active = Some(run);
            return Vec::new();
        }
        run.waiting_for_approval = false;
        let explicitly_rejected = resolution.decision == ApprovalDecisionType::Rejected as i32;
        let mut approved = resolution.decision == ApprovalDecisionType::Approved as i32;
        if approved {
            let valid = !resolution.action_id.is_empty()
                && !resolution.call_id.is_empty()
                && !resolution.permit_id.is_empty()
                && run.context.pending_action.as_ref().is_some_and(|action| {
                    action.get("id").and_then(JsonValue::as_str)
                        == Some(resolution.call_id.as_str())
                });
            if valid {
                if let Some(action) = run.context.pending_action.as_mut() {
                    action["action_id"] = json!(resolution.action_id);
                    action["permit_id"] = json!(resolution.permit_id);
                }
                run.execution_requested = false;
                run.execution_granted = false;
            } else {
                approved = false;
            }
        }
        if !approved {
            run.context.pending_action = None;
        }
        let run_id = run.run_id.clone();
        let epoch = run.lease_epoch;
        let sequence = run.next_event_sequence;
        run.next_event_sequence += 1;
        let event = self.event_for(
            &run_id,
            epoch,
            sequence,
            if approved {
                "action.approved"
            } else {
                "action.rejected"
            },
            payload(&[("reason", &resolution.reason)]),
        );
        if approved {
            self.active = Some(run);
            vec![event]
        } else {
            let (status, reason) = if explicitly_rejected {
                let reason = if resolution.reason.trim().is_empty() {
                    "Action approval was not granted. Provide new instructions to continue."
                        .to_string()
                } else {
                    resolution.reason.clone()
                };
                (RunTerminalStatus::Completed, reason)
            } else {
                (RunTerminalStatus::Failed, resolution.reason.clone())
            };
            let finish = self.message(runner_message::Payload::RunFinished(RunFinished {
                run_id,
                lease_epoch: epoch,
                status: status as i32,
                reason,
            }));
            vec![event, finish]
        }
    }
    pub fn acknowledge_events(
        &mut self,
        acknowledgement: &AcknowledgeEvents,
    ) -> Vec<RunnerMessage> {
        let Some(mut run) = self.active.take() else {
            return Vec::new();
        };
        let valid = acknowledgement.run_id == run.run_id
            && acknowledgement.execution_granted
            && run.context.pending_action.as_ref().is_some_and(|action| {
                action.get("action_id").and_then(JsonValue::as_str)
                    == Some(acknowledgement.action_id.as_str())
                    && action.get("id").and_then(JsonValue::as_str)
                        == Some(acknowledgement.call_id.as_str())
            });
        if valid {
            run.execution_granted = true;
            run.execution_requested = true;
        }
        self.active = Some(run);
        Vec::new()
    }

    pub fn cancel(&mut self, cancel: &CancelRun) -> Vec<RunnerMessage> {
        let Some(run) = self.active.as_ref() else {
            return Vec::new();
        };
        if cancel.run_id != run.run_id || cancel.lease_epoch != run.lease_epoch {
            return Vec::new();
        }
        run.cancel.cancel();
        let run_id = run.run_id.clone();
        let epoch = run.lease_epoch;
        let message = self.message(runner_message::Payload::RunFinished(RunFinished {
            run_id,
            lease_epoch: epoch,
            status: RunTerminalStatus::Canceled as i32,
            reason: cancel.reason.clone(),
        }));
        self.active = None;
        vec![message]
    }

    pub fn suspend(
        &mut self,
        suspend: &protocol::gantry::runner::v1::SuspendRun,
    ) -> Vec<RunnerMessage> {
        let Some(mut run) = self.active.take() else {
            return Vec::new();
        };
        if run.run_id != suspend.run_id || run.lease_epoch != suspend.lease_epoch {
            self.active = Some(run);
            return Vec::new();
        }
        run.suspended = true;
        let event = self.event(
            &mut run,
            "run.suspended",
            payload(&[("reason", &suspend.reason)]),
        );
        self.active = Some(run);
        vec![event]
    }

    pub fn resume_action(
        &mut self,
        resume: &protocol::gantry::runner::v1::ResumeAction,
    ) -> Vec<RunnerMessage> {
        let Some(mut run) = self.active.take() else {
            return Vec::new();
        };
        if run.run_id != resume.run_id || run.lease_epoch != resume.lease_epoch {
            self.active = Some(run);
            return Vec::new();
        }
        run.suspended = false;
        let event = self.event(
            &mut run,
            "run.resumed",
            payload(&[("action_id", &resume.action_id)]),
        );
        self.active = Some(run);
        vec![event]
    }

    async fn dispatch_tool(
        &self,
        run: &ActiveRun,
        name: &str,
        arguments: JsonValue,
    ) -> crate::tools::ToolResult {
        if !run.manifest.tools.is_empty()
            && !run.manifest.tools.iter().any(|enabled| enabled == name)
        {
            return crate::tools::ToolResult {
                ok: false,
                content: format!("tool is disabled by manifest: {name}"),
                snapshot: None,
                warnings: vec![],
            };
        }
        match name {
            "read" => run.tools.read(
                arguments
                    .get("path")
                    .and_then(JsonValue::as_str)
                    .unwrap_or_default(),
            ),
            "grep" => run.tools.grep(
                arguments
                    .get("pattern")
                    .and_then(JsonValue::as_str)
                    .unwrap_or_default(),
                arguments.get("glob").and_then(JsonValue::as_str),
            ),
            "glob" => run.tools.glob(
                arguments
                    .get("pattern")
                    .and_then(JsonValue::as_str)
                    .unwrap_or_default(),
            ),
            "write" => run.tools.write(
                arguments
                    .get("path")
                    .and_then(JsonValue::as_str)
                    .unwrap_or_default(),
                arguments
                    .get("content")
                    .and_then(JsonValue::as_str)
                    .unwrap_or_default(),
            ),
            "edit" => {
                let replacements = arguments
                    .get("replacements")
                    .and_then(JsonValue::as_array)
                    .map(|items| {
                        items
                            .iter()
                            .filter_map(|item| {
                                Some((
                                    item.get("line")?.as_u64()? as usize,
                                    item.get("content")?.as_str()?.to_string(),
                                ))
                            })
                            .collect::<Vec<_>>()
                    })
                    .unwrap_or_default();
                run.tools.hashline_edit(
                    arguments
                        .get("path")
                        .and_then(JsonValue::as_str)
                        .unwrap_or_default(),
                    arguments
                        .get("tag")
                        .and_then(JsonValue::as_str)
                        .unwrap_or_default(),
                    &replacements,
                )
            }
            "shell" => {
                let command = arguments
                    .get("command")
                    .and_then(JsonValue::as_str)
                    .unwrap_or_default();
                match check_command(
                    command,
                    run.manifest.command_policy.allow_shell,
                    &run.manifest.command_policy.denied_patterns,
                    &run.manifest.command_policy.interceptor_patterns,
                ) {
                    Ok(()) => match pty_handler::run_command_with_cancel_bounded(
                        command,
                        std::path::Path::new(&run.manifest.workspace_root),
                        Duration::from_secs(run.manifest.limits.timeout_seconds.max(1)),
                        run.cancel.clone(),
                        run.manifest.limits.max_output_bytes,
                    )
                    .await
                    {
                        Ok(content) => crate::tools::ToolResult {
                            ok: true,
                            content,
                            snapshot: None,
                            warnings: vec![],
                        },
                        Err(error) => crate::tools::ToolResult {
                            ok: false,
                            content: error.to_string(),
                            snapshot: None,
                            warnings: vec![],
                        },
                    },
                    Err(error) => crate::tools::ToolResult {
                        ok: false,
                        content: error.to_string(),
                        snapshot: None,
                        warnings: vec![],
                    },
                }
            }
            _ => crate::tools::ToolResult {
                ok: false,
                content: format!("unknown tool: {name}"),
                snapshot: None,
                warnings: vec![],
            },
        }
    }

    fn event(&mut self, run: &mut ActiveRun, event_type: &str, payload: Struct) -> RunnerMessage {
        let sequence = run.next_event_sequence;
        run.next_event_sequence += 1;
        self.event_for(&run.run_id, run.lease_epoch, sequence, event_type, payload)
    }
    fn event_for(
        &mut self,
        run_id: &str,
        lease_epoch: u64,
        sequence: u64,
        event_type: &str,
        payload: Struct,
    ) -> RunnerMessage {
        self.message(runner_message::Payload::EventBatch(RunEventBatch {
            run_id: run_id.into(),
            lease_epoch,
            events: vec![RunEvent {
                client_sequence: sequence,
                event_type: event_type.into(),
                occurred_at: None,
                payload: Some(payload),
            }],
        }))
    }
    fn finish_message(
        &mut self,
        run: &ActiveRun,
        status: RunTerminalStatus,
        reason: &str,
    ) -> RunnerMessage {
        self.message(runner_message::Payload::RunFinished(RunFinished {
            run_id: run.run_id.clone(),
            lease_epoch: run.lease_epoch,
            status: status as i32,
            reason: reason.into(),
        }))
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

fn payload(values: &[(&str, &str)]) -> Struct {
    let fields = values
        .iter()
        .map(|(key, value)| {
            (
                (*key).into(),
                Value {
                    kind: Some(Kind::StringValue((*value).into())),
                },
            )
        })
        .collect();
    Struct { fields }
}

fn artifact_declarations(
    manifest: &RunManifest,
    run_id: &str,
    lease_epoch: u64,
    sources: &mut HashMap<String, String>,
) -> Vec<ArtifactDeclaration> {
    let Ok(root) = std::fs::canonicalize(&manifest.workspace_root) else {
        return Vec::new();
    };
    manifest
        .artifacts
        .iter()
        .enumerate()
        .filter_map(|(index, spec)| {
            let candidate = root.join(&spec.path);
            let path = std::fs::canonicalize(candidate).ok()?;
            if !path.starts_with(&root) {
                return None;
            }
            let bytes = std::fs::read(&path).ok()?;
            let artifact_id = format!("{}-artifact-{}", run_id, index + 1);
            let filename = if spec.filename.trim().is_empty() {
                path.file_name()?.to_string_lossy().into_owned()
            } else {
                spec.filename.clone()
            };
            sources.insert(artifact_id.clone(), path.to_string_lossy().into_owned());
            Some(ArtifactDeclaration {
                run_id: run_id.into(),
                lease_epoch,
                artifact_id,
                filename,
                media_type: spec.media_type.clone(),
                size_bytes: bytes.len() as u64,
                digest: RunManifest::digest_bytes(&bytes),
                source_path: path.to_string_lossy().into_owned(),
            })
        })
        .collect()
}

fn action_proposal_payload(
    call_id: &str,
    tool_name: &str,
    operation: &str,
    effect: &str,
    arguments: &JsonValue,
) -> Struct {
    let mut fields = payload(&[
        ("call_id", call_id),
        ("tool_name", tool_name),
        ("operation", operation),
        ("effect", effect),
    ])
    .fields;
    fields.insert("arguments".into(), json_to_proto_value(arguments));
    Struct { fields }
}

fn json_to_proto_value(value: &JsonValue) -> Value {
    let kind = match value {
        JsonValue::Null => Kind::NullValue(0),
        JsonValue::Bool(value) => Kind::BoolValue(*value),
        JsonValue::Number(value) => Kind::NumberValue(value.as_f64().unwrap_or_default()),
        JsonValue::String(value) => Kind::StringValue(value.clone()),
        JsonValue::Array(values) => Kind::ListValue(prost_types::ListValue {
            values: values.iter().map(json_to_proto_value).collect(),
        }),
        JsonValue::Object(values) => Kind::StructValue(Struct {
            fields: values
                .iter()
                .map(|(key, value)| (key.clone(), json_to_proto_value(value)))
                .collect(),
        }),
    };
    Value { kind: Some(kind) }
}

fn bounded_output(run: &mut ActiveRun, text: &str) -> String {
    let limit = run.manifest.limits.max_output_bytes;
    if run.output_bytes >= limit {
        return String::new();
    }
    let remaining = limit - run.output_bytes;
    let bytes = text.as_bytes();
    let take = bytes.len().min(remaining);
    let mut end = take;
    while end > 0 && !text.is_char_boundary(end) {
        end -= 1;
    }
    let value = text[..end].to_string();
    run.output_bytes += value.len();
    value
}

fn tool_result_content(result: &crate::tools::ToolResult) -> String {
    serde_json::to_string(&json!({
        "content": result.content,
        "snapshot": result.snapshot,
        "warnings": result.warnings,
    }))
    .unwrap_or_else(|_| result.content.clone())
}

fn checkpoint_path_allowed(workspace_root: &str, checkpoint_path: &str) -> bool {
    let Ok(root) = std::fs::canonicalize(workspace_root) else {
        return false;
    };
    let candidate = root.join(checkpoint_path);
    let Some(parent) = candidate.parent() else {
        return false;
    };
    std::fs::canonicalize(parent)
        .map(|canonical_parent| canonical_parent.starts_with(&root))
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;
    use protocol::types::{CheckpointConfig, CommandPolicy, ModelConfig, ResourceLimits};

    fn assignment(user_input: &str) -> AssignRun {
        let manifest = RunManifest {
            kind: RUNNER_MANIFEST_KIND.into(),
            model: ModelConfig::default(),
            system_prompt: String::new(),
            user_input: user_input.into(),
            rules: Vec::new(),
            tools: Vec::new(),
            workspace_root: ".".into(),
            limits: ResourceLimits::default(),
            checkpoint: CheckpointConfig::default(),
            command_policy: CommandPolicy::default(),
            artifacts: Vec::new(),
        };
        AssignRun {
            run_id: "run-1".into(),
            lease_epoch: 1,
            manifest: serde_json::to_vec(&manifest).unwrap(),
            manifest_digest: manifest.digest().unwrap(),
            assignment_expiry: None,
        }
    }

    fn shell_assignment() -> AssignRun {
        let manifest = RunManifest {
            kind: RUNNER_MANIFEST_KIND.into(),
            model: ModelConfig::default(),
            system_prompt: String::new(),
            user_input: "shell echo approved".into(),
            rules: Vec::new(),
            tools: vec!["shell".into()],
            workspace_root: ".".into(),
            limits: ResourceLimits::default(),
            checkpoint: CheckpointConfig::default(),
            command_policy: CommandPolicy {
                allow_shell: true,
                ..Default::default()
            },
            artifacts: Vec::new(),
        };
        AssignRun {
            run_id: "run-shell".into(),
            lease_epoch: 1,
            manifest: serde_json::to_vec(&manifest).unwrap(),
            manifest_digest: manifest.digest().unwrap(),
            assignment_expiry: None,
        }
    }

    #[tokio::test]
    async fn scripted_manifest_emits_model_event_and_finishes() {
        let mut executor = AgentExecutor::new("runner-1");
        assert_eq!(executor.assign(&assignment("hello")).len(), 1);
        let events = executor.tick().await;
        assert!(events.iter().any(|message| matches!(
            message.payload,
            Some(runner_message::Payload::EventBatch(_))
        )));
        assert!(events.iter().any(|message| matches!(
            message.payload,
            Some(runner_message::Payload::RunFinished(_))
        )));
    }

    #[test]
    fn invalid_workspace_assignment_finishes_as_failed() {
        let mut assignment = assignment("hello");
        let mut manifest = serde_json::from_slice::<RunManifest>(&assignment.manifest).unwrap();
        manifest.workspace_root = "gantry-missing-workspace".into();
        assignment.manifest = serde_json::to_vec(&manifest).unwrap();
        assignment.manifest_digest = manifest.digest().unwrap();

        let messages = AgentExecutor::new("runner-1").assign(&assignment);
        let finished = messages
            .iter()
            .find_map(|message| match message.payload.as_ref() {
                Some(runner_message::Payload::RunFinished(finished)) => Some(finished),
                _ => None,
            })
            .expect("invalid assignment must reach a terminal state");
        assert_eq!(finished.run_id, "run-1");
        assert_eq!(finished.lease_epoch, 1);
        assert_eq!(finished.status, RunTerminalStatus::Failed as i32);
    }

    #[tokio::test]
    async fn await_cancel_manifest_remains_active() {
        let mut executor = AgentExecutor::new("runner-1");
        executor.assign(&assignment("wait for cancellation"));
        let events = executor.tick().await;
        assert!(events.iter().all(|message| !matches!(
            message.payload,
            Some(runner_message::Payload::RunFinished(_))
        )));
        assert!(
            !executor
                .cancel(&CancelRun {
                    run_id: "run-1".into(),
                    lease_epoch: 1,
                    reason: "test".into(),
                    force: false
                })
                .is_empty()
        );
    }

    #[tokio::test]
    async fn approved_shell_action_is_executed_before_final_response() {
        let mut executor = AgentExecutor::new("runner-1");
        assert_eq!(executor.assign(&shell_assignment()).len(), 1);
        let proposed = executor.tick().await;
        assert!(proposed.iter().any(|message| {
            message.payload.as_ref().is_some_and(|payload| {
                matches!(payload, runner_message::Payload::EventBatch(batch)
                    if batch.events.iter().any(|event| event.event_type == "action.proposed"))
            })
        }));
        let proposal_event = proposed
            .iter()
            .find_map(|message| match message.payload.as_ref() {
                Some(runner_message::Payload::EventBatch(batch)) => batch
                    .events
                    .iter()
                    .find(|event| event.event_type == "action.proposed"),
                _ => None,
            })
            .expect("action proposal event");
        let proposal_payload = proposal_event.payload.as_ref().expect("proposal payload");
        assert!(matches!(
            proposal_payload.fields.get("call_id").and_then(|value| value.kind.as_ref()),
            Some(Kind::StringValue(value)) if value == "scripted-shell"
        ));
        let arguments = match proposal_payload
            .fields
            .get("arguments")
            .and_then(|value| value.kind.as_ref())
        {
            Some(Kind::StructValue(arguments)) => arguments,
            _ => panic!("proposal arguments are not a struct"),
        };
        assert!(matches!(
            arguments.fields.get("command").and_then(|value| value.kind.as_ref()),
            Some(Kind::StringValue(value)) if value == "echo approved"
        ));

        let approved = executor.resolve_approval(&ApprovalResolution {
            run_id: "run-shell".into(),
            approval_request_id: "approval-1".into(),
            decision: ApprovalDecisionType::Approved as i32,
            reason: String::new(),
            lease_epoch: 1,
            action_id: "action-1".into(),
            call_id: "scripted-shell".into(),
            permit_id: "permit-1".into(),
            permit_expires_at: None,
        });
        assert!(!approved.is_empty());
        let request = executor.tick().await;
        assert!(request.iter().any(|message| {
            message.payload.as_ref().is_some_and(|payload| {
                matches!(payload, runner_message::Payload::EventBatch(batch)
                    if batch.events.iter().any(|event| event.event_type == "action.execution_requested"))
            })
        }));
        assert!(
            executor
                .acknowledge_events(&AcknowledgeEvents {
                    run_id: "run-shell".into(),
                    last_acknowledged_sequence: 4,
                    execution_granted: true,
                    action_id: "action-1".into(),
                    call_id: "scripted-shell".into(),
                })
                .is_empty()
        );
        let tool_result = executor.tick().await;
        assert!(tool_result.iter().any(|message| {
            message.payload.as_ref().is_some_and(|payload| {
                matches!(payload, runner_message::Payload::EventBatch(batch)
                    if batch.events.iter().any(|event| event.event_type == "tool.call.failed" || event.event_type == "tool.call.completed"))
            })
        }));
        let terminal_event = tool_result
            .iter()
            .find_map(|message| match message.payload.as_ref() {
                Some(runner_message::Payload::EventBatch(batch)) => {
                    batch.events.iter().find(|event| {
                        event.event_type == "tool.call.failed"
                            || event.event_type == "tool.call.completed"
                    })
                }
                _ => None,
            })
            .expect("terminal tool event");
        assert!(matches!(
            terminal_event
                .payload
                .as_ref()
                .and_then(|payload| payload.fields.get("call_id"))
                .and_then(|value| value.kind.as_ref()),
            Some(Kind::StringValue(value)) if value == "scripted-shell"
        ));
        let finished = executor.tick().await;
        assert!(finished.iter().any(|message| matches!(
            message.payload,
            Some(runner_message::Payload::RunFinished(_))
        )));
    }

    #[tokio::test]
    async fn rejected_action_completes_at_the_approval_boundary_for_requester_input() {
        let mut executor = AgentExecutor::new("runner-1");
        assert_eq!(executor.assign(&shell_assignment()).len(), 1);
        let proposed = executor.tick().await;
        assert!(proposed.iter().any(|message| matches!(
            message.payload.as_ref(),
            Some(runner_message::Payload::EventBatch(batch))
                if batch.events.iter().any(|event| event.event_type == "action.proposed")
        )));

        let resolved = executor.resolve_approval(&ApprovalResolution {
            run_id: "run-shell".into(),
            approval_request_id: "approval-1".into(),
            decision: ApprovalDecisionType::Rejected as i32,
            reason: "Use a different target".into(),
            lease_epoch: 1,
            action_id: "action-1".into(),
            call_id: "scripted-shell".into(),
            permit_id: String::new(),
            permit_expires_at: None,
        });
        assert!(resolved.iter().any(|message| matches!(
            message.payload.as_ref(),
            Some(runner_message::Payload::EventBatch(batch))
                if batch.events.iter().any(|event| event.event_type == "action.rejected")
        )));
        let finished = resolved
            .iter()
            .find_map(|message| match message.payload.as_ref() {
                Some(runner_message::Payload::RunFinished(finished)) => Some(finished),
                _ => None,
            })
            .expect("rejected action finishes the Run");
        assert_eq!(
            finished.status,
            RunTerminalStatus::Completed as i32,
            "approval rejection is not a Run failure"
        );
        assert_eq!(finished.reason, "Use a different target");
        assert!(executor.tick().await.is_empty());
    }

    #[tokio::test]
    async fn invalid_approved_resolution_remains_a_run_failure() {
        let mut executor = AgentExecutor::new("runner-1");
        executor.assign(&shell_assignment());
        executor.tick().await;

        let resolved = executor.resolve_approval(&ApprovalResolution {
            run_id: "run-shell".into(),
            approval_request_id: "approval-1".into(),
            decision: ApprovalDecisionType::Approved as i32,
            reason: "missing execution permit".into(),
            lease_epoch: 1,
            action_id: "action-1".into(),
            call_id: "scripted-shell".into(),
            permit_id: String::new(),
            permit_expires_at: None,
        });
        let finished = resolved
            .iter()
            .find_map(|message| match message.payload.as_ref() {
                Some(runner_message::Payload::RunFinished(finished)) => Some(finished),
                _ => None,
            })
            .expect("invalid approved resolution finishes the Run");
        assert_eq!(finished.status, RunTerminalStatus::Failed as i32);
    }

    #[tokio::test]
    async fn suspend_and_resume_fence_execution_to_the_current_lease() {
        let mut executor = AgentExecutor::new("runner-1");
        let assignment = assignment("hello");
        executor.assign(&assignment);
        let suspended = executor.suspend(&protocol::gantry::runner::v1::SuspendRun {
            run_id: "run-1".into(),
            lease_epoch: 1,
            reason: "pause".into(),
        });
        assert_eq!(suspended.len(), 1);
        assert!(executor.tick().await.is_empty());
        assert!(
            executor
                .suspend(&protocol::gantry::runner::v1::SuspendRun {
                    run_id: "run-1".into(),
                    lease_epoch: 2,
                    reason: "stale".into(),
                })
                .is_empty()
        );
        let resumed = executor.resume_action(&protocol::gantry::runner::v1::ResumeAction {
            run_id: "run-1".into(),
            lease_epoch: 1,
            action_id: "resume-1".into(),
        });
        assert_eq!(resumed.len(), 1);
        assert!(!executor.tick().await.is_empty());
    }
}
