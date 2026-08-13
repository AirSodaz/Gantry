use anyhow::Result;
use protocol::gantry::runner::v1::{
    Heartbeat, RegisterRunner, RunnerMessage, runner_message,
    runner_session_client::RunnerSessionClient,
};
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
    let runner_id = env::var("GANTRY_RUNNER_ID").unwrap_or_else(|_| "dev-runner-01".into());
    let channel = connect(address).await?;
    let mut client = RunnerSessionClient::new(channel);
    let (sender, receiver) = mpsc::channel(32);
    sender.send(register(&runner_id)).await?;
    let response = client.session(ReceiverStream::new(receiver)).await?;
    let mut inbound = response.into_inner();
    let heartbeat_sender = sender.clone();
    let heartbeat_id = runner_id.clone();
    let heartbeat = tokio::spawn(async move {
        let mut interval = tokio::time::interval(std::time::Duration::from_secs(5));
        loop {
            interval.tick().await;
            if heartbeat_sender
                .send(heartbeat_message(&heartbeat_id))
                .await
                .is_err()
            {
                break;
            }
        }
    });
    loop {
        tokio::select! {
            message = inbound.message() => {
                match message? {
                    Some(message) => tracing::info!("control-plane message received: {:?}", message.payload),
                    None => break,
                }
            }
            _ = tokio::signal::ctrl_c() => break,
        }
    }
    heartbeat.abort();
    Ok(())
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

fn register(runner_id: &str) -> RunnerMessage {
    RunnerMessage {
        runner_id: runner_id.into(),
        session_id: format!("{runner_id}-session"),
        message_id: 1,
        protocol_version: 1,
        payload: Some(runner_message::Payload::Register(RegisterRunner {
            runner_version: env!("CARGO_PKG_VERSION").into(),
            capabilities: vec!["phase0.session".into()],
            organization_id: "dev".into(),
            resource_limits: None,
        })),
    }
}

fn heartbeat_message(runner_id: &str) -> RunnerMessage {
    RunnerMessage {
        runner_id: runner_id.into(),
        session_id: format!("{runner_id}-session"),
        message_id: 2,
        protocol_version: 1,
        payload: Some(runner_message::Payload::Heartbeat(Heartbeat {
            timestamp: None,
            run_id: String::new(),
            lease_epoch: 0,
            status: 1,
        })),
    }
}
