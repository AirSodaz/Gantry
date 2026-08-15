use aes_gcm::{
    Aes256Gcm, KeyInit, Nonce,
    aead::{Aead, OsRng, rand_core::RngCore},
};
use anyhow::{Context, Result, anyhow};
use protocol::types::RunManifest;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    fs,
    path::{Path, PathBuf},
};

use crate::context::ContextState;

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct Checkpoint {
    pub schema_version: u32,
    pub run_id: String,
    pub lease_epoch: u64,
    pub manifest_digest: String,
    pub context_digest: String,
    pub context: ContextState,
}

pub struct CheckpointStore {
    path: PathBuf,
    key: [u8; 32],
}

impl CheckpointStore {
    pub fn from_env(path: impl AsRef<Path>) -> Result<Self> {
        let raw = std::env::var("GANTRY_RUNNER_CHECKPOINT_KEY")
            .context("GANTRY_RUNNER_CHECKPOINT_KEY is required for checkpoint persistence")?;
        let mut digest = Sha256::new();
        digest.update(raw.as_bytes());
        let key: [u8; 32] = digest.finalize().into();
        Ok(Self {
            path: path.as_ref().to_path_buf(),
            key,
        })
    }

    pub fn save(
        &self,
        run_id: &str,
        lease_epoch: u64,
        manifest: &RunManifest,
        mut context: ContextState,
    ) -> Result<()> {
        redact_context(&mut context);
        let context_bytes = serde_json::to_vec(&context)?;
        let mut digest = Sha256::new();
        digest.update(&context_bytes);
        let checkpoint = Checkpoint {
            schema_version: 1,
            run_id: run_id.into(),
            lease_epoch,
            manifest_digest: manifest.digest().map_err(|error| anyhow!(error))?,
            context_digest: format!("sha256:{}", hex::encode(digest.finalize())),
            context,
        };
        let plaintext = serde_json::to_vec(&checkpoint)?;
        let cipher =
            Aes256Gcm::new_from_slice(&self.key).map_err(|error| anyhow!(error.to_string()))?;
        let mut nonce_bytes = [0u8; 12];
        OsRng.fill_bytes(&mut nonce_bytes);
        let ciphertext = cipher
            .encrypt(Nonce::from_slice(&nonce_bytes), plaintext.as_ref())
            .map_err(|error| anyhow!(error.to_string()))?;
        let mut bytes = nonce_bytes.to_vec();
        bytes.extend(ciphertext);
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent)?;
        }
        let temp = self.path.with_extension("tmp");
        fs::write(&temp, bytes)?;
        fs::rename(temp, &self.path)?;
        Ok(())
    }

    pub fn load(
        &self,
        run_id: &str,
        lease_epoch: u64,
        manifest: &RunManifest,
    ) -> Result<Option<ContextState>> {
        if !self.path.exists() {
            return Ok(None);
        }
        let bytes = fs::read(&self.path)?;
        if bytes.len() < 12 {
            return Err(anyhow!("checkpoint is truncated"));
        }
        let cipher =
            Aes256Gcm::new_from_slice(&self.key).map_err(|error| anyhow!(error.to_string()))?;
        let plaintext = cipher
            .decrypt(Nonce::from_slice(&bytes[..12]), &bytes[12..])
            .map_err(|error| anyhow!(error.to_string()))?;
        let checkpoint: Checkpoint = serde_json::from_slice(&plaintext)?;
        if checkpoint.schema_version != 1
            || checkpoint.run_id != run_id
            || checkpoint.lease_epoch != lease_epoch
            || checkpoint.manifest_digest != manifest.digest().map_err(|error| anyhow!(error))?
        {
            return Err(anyhow!("checkpoint identity does not match assignment"));
        }
        let context_bytes = serde_json::to_vec(&checkpoint.context)?;
        let mut digest = Sha256::new();
        digest.update(&context_bytes);
        if checkpoint.context_digest != format!("sha256:{}", hex::encode(digest.finalize())) {
            return Err(anyhow!("checkpoint context digest mismatch"));
        }
        Ok(Some(checkpoint.context))
    }
}

fn redact_context(context: &mut ContextState) {
    for message in &mut context.messages {
        redact_value(&mut message.content);
    }
    if let Some(summary) = &mut context.summary {
        *summary = redact_string(summary);
    }
    if let Some(action) = &mut context.pending_action {
        redact_value(action);
    }
}

fn redact_value(value: &mut serde_json::Value) {
    match value {
        serde_json::Value::String(text) => *text = redact_string(text),
        serde_json::Value::Array(items) => items.iter_mut().for_each(redact_value),
        serde_json::Value::Object(fields) => fields.values_mut().for_each(redact_value),
        _ => {}
    }
}

fn redact_string(text: &str) -> String {
    let mut redacted = text.to_string();
    for name in [
        "OPENAI_API_KEY",
        "ANTHROPIC_API_KEY",
        "GANTRY_RUNNER_CHECKPOINT_KEY",
        "AWS_SECRET_ACCESS_KEY",
        "GANTRY_DEV_CREDENTIAL_KEY",
    ] {
        if let Ok(secret) = std::env::var(name) {
            if !secret.is_empty() {
                redacted = redacted.replace(&secret, "[REDACTED]");
            }
        }
    }
    redacted
        .split_whitespace()
        .map(|part| {
            if part.starts_with("sk-") || part.starts_with("sk-ant-") {
                "[REDACTED]"
            } else {
                part
            }
        })
        .collect::<Vec<_>>()
        .join(" ")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::context::ContextState;
    use protocol::types::{
        CheckpointConfig, CommandPolicy, ModelConfig, RUNNER_MANIFEST_KIND, ResourceLimits,
    };

    #[test]
    fn encrypted_checkpoint_round_trips() {
        unsafe {
            std::env::set_var("GANTRY_RUNNER_CHECKPOINT_KEY", "test-key");
        }
        let path = std::env::temp_dir().join(format!("gantry-checkpoint-{}", uuid::Uuid::new_v4()));
        let store = CheckpointStore::from_env(&path).unwrap();
        let manifest = RunManifest {
            kind: RUNNER_MANIFEST_KIND.into(),
            model: ModelConfig::default(),
            system_prompt: String::new(),
            user_input: "test".into(),
            rules: vec![],
            tools: vec![],
            workspace_root: std::env::temp_dir().to_string_lossy().into(),
            limits: ResourceLimits::default(),
            checkpoint: CheckpointConfig {
                enabled: true,
                path: Some(path.to_string_lossy().into()),
            },
            command_policy: CommandPolicy::default(),
            artifacts: Vec::new(),
        };
        store
            .save("run", 1, &manifest, ContextState::default())
            .unwrap();
        assert_eq!(
            store.load("run", 1, &manifest).unwrap().unwrap(),
            ContextState::default()
        );
    }

    #[test]
    fn checkpoint_rejects_wrong_epoch_and_key() {
        unsafe {
            std::env::set_var("GANTRY_RUNNER_CHECKPOINT_KEY", "checkpoint-key-a");
        }
        let path = std::env::temp_dir().join(format!("gantry-checkpoint-{}", uuid::Uuid::new_v4()));
        let store = CheckpointStore::from_env(&path).unwrap();
        let manifest = RunManifest {
            kind: RUNNER_MANIFEST_KIND.into(),
            model: ModelConfig::default(),
            system_prompt: String::new(),
            user_input: "test".into(),
            rules: vec![],
            tools: vec![],
            workspace_root: std::env::temp_dir().to_string_lossy().into(),
            limits: ResourceLimits::default(),
            checkpoint: CheckpointConfig {
                enabled: true,
                path: Some(path.to_string_lossy().into()),
            },
            command_policy: CommandPolicy::default(),
            artifacts: Vec::new(),
        };
        store
            .save("run", 1, &manifest, ContextState::default())
            .unwrap();
        assert!(store.load("run", 2, &manifest).is_err());
        unsafe {
            std::env::set_var("GANTRY_RUNNER_CHECKPOINT_KEY", "checkpoint-key-b");
        }
        let wrong_key = CheckpointStore::from_env(&path).unwrap();
        assert!(wrong_key.load("run", 1, &manifest).is_err());
    }
}
