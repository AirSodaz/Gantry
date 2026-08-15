use anyhow::Result;
use protocol::gantry::runner::v1::{
    control_plane_message, runner_session_client::RunnerSessionClient,
};
mod checkpoint;
mod context;
mod diagnostics;
mod executor;
mod model;
mod rules;
mod tools;
use executor::AgentExecutor;
use std::env;
use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;
use tonic::transport::{Certificate, Channel, ClientTlsConfig, Endpoint, Identity};

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(env::var("GANTRY_LOG_LEVEL").unwrap_or_else(|_| "info".into()))
        .init();
    let address =
        env::var("GANTRY_CONTROL_PLANE_ADDR").unwrap_or_else(|_| "http://127.0.0.1:8081".into());
    let http_address = env::var("GANTRY_CONTROL_PLANE_HTTP_ADDR")
        .unwrap_or_else(|_| "http://127.0.0.1:8080".into());
    let runner_id = env::var("GANTRY_RUNNER_ID").unwrap_or_else(|_| "dev-runner-01".into());
    loop {
        match session(address.clone(), http_address.clone(), runner_id.clone()).await {
            Ok(false) => return Ok(()),
            Ok(true) => tracing::warn!("runner session closed; reconnecting"),
            Err(error) => tracing::warn!(%error, "runner session failed; reconnecting"),
        }
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
    }
}

async fn session(address: String, http_address: String, runner_id: String) -> Result<bool> {
    let channel = connect(address).await?;
    let mut client = RunnerSessionClient::new(channel);
    let (sender, receiver) = mpsc::channel(32);
    let mut executor = AgentExecutor::new(runner_id);
    sender.send(executor.register()).await?;
    let response = client.session(ReceiverStream::new(receiver)).await?;
    let mut inbound = response.into_inner();
    let mut tick = tokio::time::interval(std::time::Duration::from_millis(250));
    let mut heartbeat = tokio::time::interval(std::time::Duration::from_secs(5));
    loop {
        tokio::select! {
            message = inbound.message() => {
                match message? {
                    Some(message) => {
                        match message.payload {
                            Some(control_plane_message::Payload::AssignRun(assignment)) => {
                                for outbound in executor.assign(&assignment) { sender.send(outbound).await?; }
                            }
                            Some(control_plane_message::Payload::CancelRun(cancel)) => {
                                for outbound in executor.cancel(&cancel) { sender.send(outbound).await?; }
                            }
                            Some(control_plane_message::Payload::ApprovalResolution(resolution)) => {
                                for outbound in executor.resolve_approval(&resolution) { sender.send(outbound).await?; }
                            }
                            Some(control_plane_message::Payload::AcknowledgeEvents(acknowledgement)) => {
                                for outbound in executor.acknowledge_events(&acknowledgement) { sender.send(outbound).await?; }
                            }
                            Some(control_plane_message::Payload::ArtifactUploadGrant(grant)) => {
                                if let Some(outbound) = executor.upload_artifact(&grant, &http_address).await {
                                    sender.send(outbound).await?;
                                }
                            }
                            Some(control_plane_message::Payload::SuspendRun(suspend)) => {
                                for outbound in executor.suspend(&suspend) { sender.send(outbound).await?; }
                            }
                            Some(control_plane_message::Payload::ResumeAction(resume)) => {
                                for outbound in executor.resume_action(&resume) { sender.send(outbound).await?; }
                            }
                            payload => tracing::info!("control-plane message received: {:?}", payload),
                        }
                    }
                    None => return Ok(true),
                }
            }
            _ = tick.tick() => { for outbound in executor.tick().await { sender.send(outbound).await?; } }
            _ = heartbeat.tick() => { sender.send(executor.heartbeat()).await?; }
            _ = tokio::signal::ctrl_c() => return Ok(false),
        }
    }
}

async fn connect(address: String) -> Result<Channel> {
    let is_local_plaintext = address.starts_with("http://");
    let mut endpoint = Endpoint::from_shared(address)?;
    let ca = env::var("GANTRY_RUNNER_CA_FILE").ok();
    let cert = env::var("GANTRY_RUNNER_CERT_FILE").ok();
    let key = env::var("GANTRY_RUNNER_KEY_FILE").ok();
    match (ca, cert, key) {
        (Some(ca), Some(cert), Some(key)) => {
            let tls = ClientTlsConfig::new()
                .ca_certificate(Certificate::from_pem(std::fs::read(ca)?))
                .identity(Identity::from_pem(
                    std::fs::read(cert)?,
                    std::fs::read(key)?,
                ));
            endpoint = endpoint.tls_config(tls)?;
        }
        (None, None, None) if is_local_plaintext => {}
        _ => anyhow::bail!(
            "set all GANTRY_RUNNER_CA_FILE, GANTRY_RUNNER_CERT_FILE, and GANTRY_RUNNER_KEY_FILE, or use http:// only for local development"
        ),
    }
    Ok(endpoint.connect().await?)
}
