use anyhow::{Context, Result, anyhow};
use async_trait::async_trait;
use futures_util::StreamExt;
use protocol::types::{ModelConfig, RunManifest};
use serde_json::{Value, json};
use std::{collections::HashMap, time::Duration};
use tokio_util::sync::CancellationToken;

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ModelEvent {
    TextDelta(String),
    ThinkingDelta(String),
    ToolCallDelta {
        id: String,
        name: String,
        arguments: String,
    },
    Usage {
        input_tokens: usize,
        output_tokens: usize,
    },
    Wait,
    Done,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum RetryClass {
    Retryable,
    NonRetryable,
    Cancelled,
    Timeout,
}

pub fn classify_http_status(status: u16) -> RetryClass {
    match status {
        408 | 409 | 425 | 429 | 500..=599 => RetryClass::Retryable,
        _ => RetryClass::NonRetryable,
    }
}

#[async_trait]
pub trait ModelClient: Send + Sync {
    async fn stream(
        &self,
        manifest: &RunManifest,
        prompt: &[Value],
        cancel: CancellationToken,
    ) -> Result<Vec<ModelEvent>>;
}

pub fn from_config(config: &ModelConfig) -> Result<Box<dyn ModelClient>> {
    if config.provider != "scripted"
        && std::env::var("GANTRY_ALLOW_DIRECT_MODEL").as_deref() != Ok("1")
    {
        return Err(anyhow!(
            "direct provider mode is disabled; set GANTRY_ALLOW_DIRECT_MODEL=1 for development"
        ));
    }
    match config.provider.as_str() {
        "scripted" => Ok(Box::new(ScriptedModel)),
        "openai" | "openai-compatible" => Ok(Box::new(OpenAiModel::new(config.clone())?)),
        "anthropic" => Ok(Box::new(AnthropicModel::new(config.clone())?)),
        other => Err(anyhow!("unsupported model provider: {other}")),
    }
}

struct ScriptedModel;

#[async_trait]
impl ModelClient for ScriptedModel {
    async fn stream(
        &self,
        manifest: &RunManifest,
        prompt: &[Value],
        cancel: CancellationToken,
    ) -> Result<Vec<ModelEvent>> {
        if cancel.is_cancelled() {
            return Err(anyhow!("model request cancelled"));
        }
        if prompt
            .iter()
            .any(|message| message.get("role").and_then(Value::as_str) == Some("tool"))
        {
            return Ok(vec![
                ModelEvent::TextDelta("Scripted tool loop completed".into()),
                ModelEvent::Done,
            ]);
        }
        let input = manifest.user_input.to_lowercase();
        if input == "wait for cancellation" {
            return Ok(vec![ModelEvent::Wait, ModelEvent::Done]);
        }
        if let Some(path) = input.strip_prefix("read ") {
            return Ok(vec![
                ModelEvent::ToolCallDelta {
                    id: "scripted-read".into(),
                    name: "read".into(),
                    arguments: serde_json::to_string(&json!({"path": path.trim()}))?,
                },
                ModelEvent::Done,
            ]);
        }
        if let Some(command) = input.strip_prefix("shell ") {
            return Ok(vec![
                ModelEvent::ToolCallDelta {
                    id: "scripted-shell".into(),
                    name: "shell".into(),
                    arguments: serde_json::to_string(&json!({"command": command.trim()}))?,
                },
                ModelEvent::Done,
            ]);
        }
        Ok(vec![
            ModelEvent::TextDelta(format!("Scripted response: {}", manifest.user_input)),
            ModelEvent::Done,
        ])
    }
}

struct OpenAiModel {
    config: ModelConfig,
    client: reqwest::Client,
}

impl OpenAiModel {
    fn new(config: ModelConfig) -> Result<Self> {
        Ok(Self {
            config,
            client: reqwest::Client::builder().build()?,
        })
    }
}

#[async_trait]
impl ModelClient for OpenAiModel {
    async fn stream(
        &self,
        manifest: &RunManifest,
        prompt: &[Value],
        cancel: CancellationToken,
    ) -> Result<Vec<ModelEvent>> {
        let key = std::env::var("OPENAI_API_KEY").context("OPENAI_API_KEY is required")?;
        let base = self
            .config
            .base_url
            .clone()
            .unwrap_or_else(|| "https://api.openai.com/v1".into());
        let url = format!("{}/chat/completions", base.trim_end_matches('/'));
        let body = json!({
            "model": self.config.model,
            "messages": prompt,
            "tools": openai_tools(manifest),
            "stream": true
        });
        let request = self.client.post(url).bearer_auth(key).json(&body).send();
        let response = tokio::select! {
            response = request => response?,
            _ = cancel.cancelled() => return Err(anyhow!("model request cancelled")),
        };
        if !response.status().is_success() {
            return Err(anyhow!("openai request failed: {}", response.status()));
        }
        parse_sse(response, cancel, false).await
    }
}

struct AnthropicModel {
    config: ModelConfig,
    client: reqwest::Client,
}

impl AnthropicModel {
    fn new(config: ModelConfig) -> Result<Self> {
        Ok(Self {
            config,
            client: reqwest::Client::builder().build()?,
        })
    }
}

#[async_trait]
impl ModelClient for AnthropicModel {
    async fn stream(
        &self,
        manifest: &RunManifest,
        prompt: &[Value],
        cancel: CancellationToken,
    ) -> Result<Vec<ModelEvent>> {
        let key = std::env::var("ANTHROPIC_API_KEY").context("ANTHROPIC_API_KEY is required")?;
        let base = self
            .config
            .base_url
            .clone()
            .unwrap_or_else(|| "https://api.anthropic.com/v1".into());
        let url = format!("{}/messages", base.trim_end_matches('/'));
        let system = if manifest.system_prompt.is_empty() {
            None
        } else {
            Some(manifest.system_prompt.clone())
        };
        let body = json!({
            "model": self.config.model,
            "max_tokens": 4096,
            "system": system,
            "messages": prompt,
            "tools": anthropic_tools(manifest),
            "stream": true
        });
        let request = self
            .client
            .post(url)
            .header("x-api-key", key)
            .header("anthropic-version", "2023-06-01")
            .json(&body)
            .send();
        let response = tokio::select! {
            response = request => response?,
            _ = cancel.cancelled() => return Err(anyhow!("model request cancelled")),
        };
        if !response.status().is_success() {
            return Err(anyhow!("anthropic request failed: {}", response.status()));
        }
        parse_sse(response, cancel, true).await
    }
}

fn openai_tools(manifest: &RunManifest) -> Vec<Value> {
    manifest
        .tools
        .iter()
        .map(|name| {
            json!({
                "type": "function",
                "function": {
                    "name": name,
                    "description": format!("Gantry native {name} tool"),
                    "parameters": {"type": "object", "additionalProperties": true}
                }
            })
        })
        .collect()
}

fn anthropic_tools(manifest: &RunManifest) -> Vec<Value> {
    manifest
        .tools
        .iter()
        .map(|name| {
            json!({
                "name": name,
                "description": format!("Gantry native {name} tool"),
                "input_schema": {"type": "object", "additionalProperties": true}
            })
        })
        .collect()
}

async fn parse_sse(
    response: reqwest::Response,
    cancel: CancellationToken,
    anthropic: bool,
) -> Result<Vec<ModelEvent>> {
    let mut stream = response.bytes_stream();
    let mut buffer = String::new();
    let mut events = Vec::new();
    let mut openai_calls = HashMap::new();
    let mut anthropic_call: Option<AnthropicCall> = None;
    let deadline = tokio::time::sleep(Duration::from_secs(10));
    tokio::pin!(deadline);
    while let Some(chunk) = tokio::select! {
        chunk = stream.next() => chunk,
        _ = cancel.cancelled() => return Err(anyhow!("model stream cancelled")),
        _ = &mut deadline => return Err(anyhow!("model stream idle timeout")),
    } {
        let chunk = chunk?;
        buffer.push_str(std::str::from_utf8(&chunk).context("model stream was not utf8")?);
        while let Some(position) = buffer.find("\n\n") {
            let frame = buffer[..position].to_string();
            buffer.drain(..position + 2);
            let data = frame
                .lines()
                .filter_map(|line| line.strip_prefix("data:"))
                .map(str::trim)
                .collect::<String>();
            if data.is_empty() || data == "[DONE]" {
                continue;
            }
            let value: Value = serde_json::from_str(&data).context("invalid model SSE JSON")?;
            if anthropic {
                parse_anthropic_event(&value, &mut events, &mut anthropic_call);
            } else {
                parse_openai_event(&value, &mut events, &mut openai_calls);
            }
            deadline
                .as_mut()
                .reset(tokio::time::Instant::now() + Duration::from_secs(120));
        }
    }
    if !buffer.trim().is_empty() {
        let data = buffer
            .lines()
            .filter_map(|line| line.strip_prefix("data:"))
            .map(str::trim)
            .collect::<String>();
        if !data.is_empty() && data != "[DONE]" {
            let value: Value = serde_json::from_str(&data).context("invalid model SSE JSON")?;
            if anthropic {
                parse_anthropic_event(&value, &mut events, &mut anthropic_call);
            } else {
                parse_openai_event(&value, &mut events, &mut openai_calls);
            }
        }
    }
    events.push(ModelEvent::Done);
    Ok(events)
}

fn parse_openai_event(
    value: &Value,
    events: &mut Vec<ModelEvent>,
    calls: &mut HashMap<String, (String, String, bool)>,
) {
    match value.get("type").and_then(Value::as_str) {
        Some("response.output_text.delta") => {
            if let Some(text) = value.get("delta").and_then(Value::as_str) {
                events.push(ModelEvent::TextDelta(text.into()));
            }
        }
        Some("response.function_call_arguments.delta") => {
            if let Some(arguments) = value.get("delta").and_then(Value::as_str) {
                let id = value
                    .get("item_id")
                    .and_then(Value::as_str)
                    .unwrap_or("responses-tool")
                    .to_string();
                let entry = calls
                    .entry(id.clone())
                    .or_insert_with(|| (String::new(), String::new(), false));
                entry.1.push_str(arguments);
                if let Some(name) = value.get("name").and_then(Value::as_str) {
                    entry.0 = name.into();
                }
                if !entry.2
                    && !entry.0.is_empty()
                    && serde_json::from_str::<Value>(&entry.1).is_ok()
                {
                    entry.2 = true;
                    events.push(ModelEvent::ToolCallDelta {
                        id,
                        name: entry.0.clone(),
                        arguments: entry.1.clone(),
                    });
                }
            }
        }
        _ => {}
    }
    let choice = value
        .get("choices")
        .and_then(Value::as_array)
        .and_then(|v| v.first());
    if let Some(delta) = choice.and_then(|v| v.get("delta")) {
        if let Some(text) = delta.get("content").and_then(Value::as_str) {
            events.push(ModelEvent::TextDelta(text.into()));
        }
        if let Some(tool_calls) = delta.get("tool_calls").and_then(Value::as_array) {
            for (index, call) in tool_calls.iter().enumerate() {
                let id = call
                    .get("id")
                    .and_then(Value::as_str)
                    .filter(|id| !id.is_empty())
                    .unwrap_or("tool-call")
                    .to_string();
                let entry = calls
                    .entry(id.clone())
                    .or_insert_with(|| (String::new(), String::new(), false));
                if let Some(name) = call
                    .get("function")
                    .and_then(|v| v.get("name"))
                    .and_then(Value::as_str)
                {
                    entry.0 = name.to_string();
                }
                if let Some(arguments) = call
                    .get("function")
                    .and_then(|v| v.get("arguments"))
                    .and_then(Value::as_str)
                {
                    entry.1.push_str(arguments);
                }
                if !entry.2
                    && !entry.0.is_empty()
                    && serde_json::from_str::<Value>(&entry.1).is_ok()
                {
                    entry.2 = true;
                    events.push(ModelEvent::ToolCallDelta {
                        id: if id == "tool-call" {
                            format!("tool-call-{index}")
                        } else {
                            id
                        },
                        name: entry.0.clone(),
                        arguments: entry.1.clone(),
                    });
                }
            }
        }
    }
    if let Some(usage) = value.get("usage") {
        events.push(ModelEvent::Usage {
            input_tokens: usage
                .get("prompt_tokens")
                .and_then(Value::as_u64)
                .unwrap_or(0) as usize,
            output_tokens: usage
                .get("completion_tokens")
                .and_then(Value::as_u64)
                .unwrap_or(0) as usize,
        });
    }
}

#[derive(Default)]
struct AnthropicCall {
    id: String,
    name: String,
    arguments: String,
    emitted: bool,
}

fn parse_anthropic_event(
    value: &Value,
    events: &mut Vec<ModelEvent>,
    current: &mut Option<AnthropicCall>,
) {
    match value
        .get("type")
        .and_then(Value::as_str)
        .unwrap_or_default()
    {
        "content_block_start" => {
            if let Some(content_block) = value.get("content_block") {
                if content_block.get("type").and_then(Value::as_str) == Some("tool_use") {
                    *current = Some(AnthropicCall {
                        id: content_block
                            .get("id")
                            .and_then(Value::as_str)
                            .unwrap_or("anthropic-tool")
                            .into(),
                        name: content_block
                            .get("name")
                            .and_then(Value::as_str)
                            .unwrap_or_default()
                            .into(),
                        ..Default::default()
                    });
                }
            }
        }
        "content_block_delta" => {
            if let Some(delta) = value.get("delta") {
                match delta
                    .get("type")
                    .and_then(Value::as_str)
                    .unwrap_or_default()
                {
                    "text_delta" => {
                        if let Some(text) = delta.get("text").and_then(Value::as_str) {
                            events.push(ModelEvent::TextDelta(text.into()));
                        }
                    }
                    "thinking_delta" => {
                        if let Some(text) = delta.get("thinking").and_then(Value::as_str) {
                            events.push(ModelEvent::ThinkingDelta(text.into()));
                        }
                    }
                    "input_json_delta" => {
                        if let Some(partial) = delta.get("partial_json").and_then(Value::as_str) {
                            if let Some(call) = current.as_mut() {
                                call.arguments.push_str(partial);
                                if !call.emitted
                                    && !call.name.is_empty()
                                    && serde_json::from_str::<Value>(&call.arguments).is_ok()
                                {
                                    call.emitted = true;
                                    events.push(ModelEvent::ToolCallDelta {
                                        id: call.id.clone(),
                                        name: call.name.clone(),
                                        arguments: call.arguments.clone(),
                                    });
                                }
                            }
                        }
                    }
                    _ => {}
                }
            }
        }
        "content_block_stop" => {
            if let Some(call) = current.as_mut() {
                if !call.emitted
                    && !call.name.is_empty()
                    && serde_json::from_str::<Value>(&call.arguments).is_ok()
                {
                    call.emitted = true;
                    events.push(ModelEvent::ToolCallDelta {
                        id: call.id.clone(),
                        name: call.name.clone(),
                        arguments: call.arguments.clone(),
                    });
                }
            }
        }
        "message_delta" => {
            if let Some(usage) = value.get("usage") {
                events.push(ModelEvent::Usage {
                    input_tokens: 0,
                    output_tokens: usage
                        .get("output_tokens")
                        .and_then(Value::as_u64)
                        .unwrap_or(0) as usize,
                });
            }
        }
        _ => {}
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn retry_classification_distinguishes_throttling_and_auth() {
        assert_eq!(classify_http_status(429), RetryClass::Retryable);
        assert_eq!(classify_http_status(503), RetryClass::Retryable);
        assert_eq!(classify_http_status(401), RetryClass::NonRetryable);
    }

    #[test]
    fn openai_tool_arguments_are_emitted_only_after_complete_json() {
        let mut events = Vec::new();
        let mut calls = HashMap::new();
        let first: Value = serde_json::from_str(
            r#"{"choices":[{"delta":{"tool_calls":[{"id":"call-1","function":{"name":"read","arguments":"{\"path\":"}}]}}]}"#,
        )
        .unwrap();
        parse_openai_event(&first, &mut events, &mut calls);
        assert!(events.is_empty());
        let second: Value = serde_json::from_str(
            r#"{"choices":[{"delta":{"tool_calls":[{"id":"call-1","function":{"arguments":"\"a.txt\"}"}}]}}]}"#,
        )
        .unwrap();
        parse_openai_event(&second, &mut events, &mut calls);
        assert!(
            matches!(events.first(), Some(ModelEvent::ToolCallDelta { name, .. }) if name == "read")
        );
    }
}
