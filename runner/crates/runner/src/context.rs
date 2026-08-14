use protocol::types::ResourceLimits;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq)]
pub struct Message {
    pub role: String,
    pub content: Value,
}

#[derive(Clone, Debug, Default, Deserialize, Serialize, PartialEq)]
pub struct ContextState {
    pub messages: Vec<Message>,
    pub summary: Option<String>,
    pub summary_digest: Option<String>,
    pub pending_action: Option<Value>,
    pub token_estimate: usize,
    pub branch: String,
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct CompactionReport {
    pub compacted: bool,
    pub warning: Option<String>,
    pub digest: Option<String>,
}

impl ContextState {
    pub fn push(&mut self, role: impl Into<String>, content: Value) {
        self.token_estimate += serde_json::to_string(&content)
            .map(|text| text.len() / 4 + 1)
            .unwrap_or(1);
        self.messages.push(Message {
            role: role.into(),
            content,
        });
    }

    pub fn compact(&mut self, limits: &ResourceLimits) -> CompactionReport {
        let limit = limits.context_soft_limit;
        if limit == 0 || self.token_estimate <= limit {
            return CompactionReport::default();
        }
        let keep = self.messages.len().min(8);
        let protected = self
            .messages
            .iter()
            .take_while(|message| message.role == "system" || message.role == "developer")
            .count();
        let split = self.messages.len().saturating_sub(keep).max(protected);
        let archived = if split > protected {
            self.messages.drain(protected..split).collect::<Vec<_>>()
        } else {
            Vec::new()
        };
        if archived.is_empty() {
            return CompactionReport {
                compacted: false,
                warning: Some("context exceeded limit but policy messages are protected".into()),
                digest: self.summary_digest.clone(),
            };
        }
        let summary = archived
            .iter()
            .map(|message| format!("{}: {}", message.role, message.content))
            .collect::<Vec<_>>()
            .join("\n");
        let summary = match &self.summary_digest {
            Some(previous) => format!("previous_summary_digest: {previous}\n{summary}"),
            None => summary,
        };
        let mut digest = Sha256::new();
        digest.update(summary.as_bytes());
        let digest = format!("sha256:{}", hex::encode(digest.finalize()));
        self.summary = Some(summary);
        self.summary_digest = Some(digest.clone());
        self.token_estimate = self
            .messages
            .iter()
            .map(|message| {
                serde_json::to_string(&message.content)
                    .map(|text| text.len() / 4 + 1)
                    .unwrap_or(1)
            })
            .sum::<usize>()
            + self
                .summary
                .as_ref()
                .map(|summary| summary.len() / 4 + 1)
                .unwrap_or(0);
        CompactionReport {
            compacted: true,
            warning: None,
            digest: Some(digest),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compaction_keeps_recent_messages_and_digest() {
        let mut state = ContextState::default();
        for index in 0..20 {
            state.push("user", Value::String(format!("message-{index}")));
        }
        let report = state.compact(&ResourceLimits {
            context_soft_limit: 10,
            ..Default::default()
        });
        assert!(report.compacted);
        assert!(state.messages.len() <= 8);
        assert!(state.summary_digest.is_some());
    }
}
