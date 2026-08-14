use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use thiserror::Error;
use tokio_util::sync::CancellationToken;

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq)]
pub struct ToolDescriptor {
    pub name: String,
    pub description: String,
    pub input_schema: Value,
}

#[derive(Clone, Debug, Error, PartialEq, Eq)]
pub enum McpError {
    #[error("MCP tool timed out")]
    Timeout,
    #[error("MCP client cancelled")]
    Cancelled,
    #[error("MCP server is unavailable")]
    Unavailable,
}

#[async_trait]
pub trait McpClient: Send + Sync {
    async fn list_tools(&self) -> Result<Vec<ToolDescriptor>, McpError>;
    async fn call_tool(
        &self,
        name: &str,
        arguments: Value,
        cancel: CancellationToken,
    ) -> Result<Value, McpError>;
}

pub struct FakeMcpClient {
    pub tools: Vec<ToolDescriptor>,
}

#[async_trait]
impl McpClient for FakeMcpClient {
    async fn list_tools(&self) -> Result<Vec<ToolDescriptor>, McpError> {
        Ok(self.tools.clone())
    }
    async fn call_tool(
        &self,
        name: &str,
        arguments: Value,
        cancel: CancellationToken,
    ) -> Result<Value, McpError> {
        if cancel.is_cancelled() {
            return Err(McpError::Cancelled);
        }
        Ok(serde_json::json!({"tool": name, "arguments": arguments, "source": "fake-mcp"}))
    }
}
