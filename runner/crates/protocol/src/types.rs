use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

pub const RUNNER_MANIFEST_KIND: &str = "gantry.agent/v1";

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
pub struct RunManifest {
    pub kind: String,
    pub model: ModelConfig,
    #[serde(default)]
    pub system_prompt: String,
    #[serde(default)]
    pub user_input: String,
    #[serde(default)]
    pub rules: Vec<RuleSnapshot>,
    #[serde(default)]
    pub tools: Vec<String>,
    pub workspace_root: String,
    #[serde(default)]
    pub limits: ResourceLimits,
    #[serde(default)]
    pub checkpoint: CheckpointConfig,
    #[serde(default)]
    pub command_policy: CommandPolicy,
    #[serde(default)]
    pub artifacts: Vec<ArtifactSpec>,
}

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
pub struct ArtifactSpec {
    pub path: String,
    #[serde(default)]
    pub filename: String,
    #[serde(default = "default_artifact_media_type")]
    pub media_type: String,
}

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
pub struct ModelConfig {
    pub provider: String,
    pub model: String,
    #[serde(default)]
    pub base_url: Option<String>,
    #[serde(default)]
    pub max_context_tokens: usize,
}

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
pub struct ResourceLimits {
    #[serde(default = "default_turns")]
    pub max_turns: u32,
    #[serde(default = "default_output_bytes")]
    pub max_output_bytes: usize,
    #[serde(default)]
    pub context_soft_limit: usize,
    #[serde(default)]
    pub timeout_seconds: u64,
}

impl Default for ResourceLimits {
    fn default() -> Self {
        Self {
            max_turns: default_turns(),
            max_output_bytes: default_output_bytes(),
            context_soft_limit: 0,
            timeout_seconds: 0,
        }
    }
}

#[derive(Clone, Debug, Default, Deserialize, Serialize, PartialEq, Eq)]
pub struct CheckpointConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub path: Option<String>,
}

#[derive(Clone, Debug, Default, Deserialize, Serialize, PartialEq, Eq)]
pub struct CommandPolicy {
    #[serde(default)]
    pub allow_shell: bool,
    #[serde(default)]
    pub interceptor_patterns: Vec<String>,
    #[serde(default)]
    pub denied_patterns: Vec<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
pub struct RuleSnapshot {
    pub name: String,
    pub content: String,
    #[serde(default)]
    pub globs: Vec<String>,
    #[serde(default)]
    pub always_apply: bool,
    #[serde(default)]
    pub condition: Option<String>,
    #[serde(default)]
    pub scope: Vec<String>,
    #[serde(default)]
    pub interrupt_mode: Option<String>,
}

impl Default for ModelConfig {
    fn default() -> Self {
        Self {
            provider: "scripted".into(),
            model: "deterministic".into(),
            base_url: None,
            max_context_tokens: 16_000,
        }
    }
}

impl RunManifest {
    pub fn validate(&self) -> Result<(), String> {
        if self.kind != RUNNER_MANIFEST_KIND {
            return Err(format!("unsupported manifest kind: {}", self.kind));
        }
        if self.workspace_root.trim().is_empty() {
            return Err("workspace_root is required".into());
        }
        if self.model.provider.trim().is_empty() || self.model.model.trim().is_empty() {
            return Err("model provider and model are required".into());
        }
        if self.limits.max_turns == 0 {
            return Err("limits.max_turns must be greater than zero".into());
        }
        if self.limits.max_output_bytes == 0 {
            return Err("limits.max_output_bytes must be greater than zero".into());
        }
        Ok(())
    }

    pub fn canonical_bytes(&self) -> Result<Vec<u8>, String> {
        self.validate()?;
        serde_json::to_vec(self).map_err(|error| error.to_string())
    }

    pub fn digest_bytes(bytes: &[u8]) -> String {
        let mut digest = Sha256::new();
        digest.update(bytes);
        format!("sha256:{}", hex::encode(digest.finalize()))
    }

    pub fn digest(&self) -> Result<String, String> {
        Ok(Self::digest_bytes(&self.canonical_bytes()?))
    }
}

fn default_turns() -> u32 {
    12
}
fn default_output_bytes() -> usize {
    128 * 1024
}

fn default_artifact_media_type() -> String {
    "application/octet-stream".into()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn manifest_is_canonical_and_content_addressed() {
        let manifest = RunManifest {
            kind: RUNNER_MANIFEST_KIND.into(),
            model: ModelConfig::default(),
            system_prompt: "system".into(),
            user_input: "hello".into(),
            rules: Vec::new(),
            tools: vec!["read".into()],
            workspace_root: "/tmp/workspace".into(),
            limits: ResourceLimits::default(),
            checkpoint: CheckpointConfig::default(),
            command_policy: CommandPolicy::default(),
            artifacts: Vec::new(),
        };
        let digest = manifest.digest().unwrap();
        assert!(digest.starts_with("sha256:"));
        assert_eq!(
            manifest,
            serde_json::from_slice(&manifest.canonical_bytes().unwrap()).unwrap()
        );
    }
}
