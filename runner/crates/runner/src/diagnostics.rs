use serde::{Deserialize, Serialize};
use std::time::{Duration, Instant};

#[derive(Debug, Default, Serialize, Deserialize)]
pub struct RunMetrics {
    pub model_requests: u64,
    pub model_events: u64,
    pub tool_calls: u64,
    pub compacted_contexts: u64,
    pub output_bytes: u64,
}

impl RunMetrics {
    pub fn model_request(&mut self) {
        self.model_requests += 1;
    }

    pub fn model_event(&mut self) {
        self.model_events += 1;
    }

    pub fn tool_call(&mut self) {
        self.tool_calls += 1;
    }

    pub fn compacted(&mut self) {
        self.compacted_contexts += 1;
    }

    pub fn output(&mut self, bytes: usize) {
        self.output_bytes = self.output_bytes.saturating_add(bytes as u64);
    }
}

pub struct Stopwatch(Instant);

impl Stopwatch {
    pub fn start() -> Self {
        Self(Instant::now())
    }

    pub fn elapsed(&self) -> Duration {
        self.0.elapsed()
    }
}
