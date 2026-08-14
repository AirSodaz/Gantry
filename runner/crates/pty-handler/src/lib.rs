use anyhow::{Result, anyhow};
use std::time::Duration;
use tokio_util::sync::CancellationToken;

/// Execute one bounded non-interactive shell command in its own process group.
/// The process-group boundary is what lets cancellation clean up descendants.
pub async fn run_command(
    command: &str,
    cwd: &std::path::Path,
    timeout: Duration,
) -> Result<String> {
    run_command_with_cancel(command, cwd, timeout, CancellationToken::new()).await
}

/// Run a command attached to a real PTY. This is kept as a separate path from
/// `run_command_with_cancel_bounded` because PTY output has terminal semantics
/// (merged stdout/stderr, line discipline, and an interactive shell state).
#[cfg(unix)]
pub async fn run_pty_command(
    command: &str,
    cwd: &std::path::Path,
    timeout: Duration,
    max_output_bytes: usize,
) -> Result<String> {
    let command = command.to_string();
    let cwd = cwd.to_path_buf();
    let (killer_tx, killer_rx) = tokio::sync::oneshot::channel();
    let mut task = tokio::task::spawn_blocking(move || {
        run_pty_blocking(&command, &cwd, max_output_bytes, killer_tx)
    });
    let mut killer = killer_rx
        .await
        .map_err(|_| anyhow!("PTY process failed before startup"))?;
    tokio::select! {
        result = &mut task => Ok(result??),
        _ = tokio::time::sleep(timeout.max(Duration::from_millis(1))) => {
            let _ = killer.kill();
            let _ = task.await;
            Err(anyhow!("PTY command timed out"))
        }
    }
}

#[cfg(not(unix))]
pub async fn run_pty_command(
    _command: &str,
    _cwd: &std::path::Path,
    _timeout: Duration,
    _max_output_bytes: usize,
) -> Result<String> {
    Err(anyhow!(
        "PTY execution is supported only on Linux in runner V1"
    ))
}

#[cfg(unix)]
fn run_pty_blocking(
    command: &str,
    cwd: &std::path::Path,
    max_output_bytes: usize,
    killer_tx: tokio::sync::oneshot::Sender<Box<dyn portable_pty::ChildKiller + Send + Sync>>,
) -> Result<String> {
    use portable_pty::{CommandBuilder, PtySize, native_pty_system};
    use std::io::Read;

    let pty = native_pty_system();
    let pair = pty.openpty(PtySize::default())?;
    let mut builder = CommandBuilder::new("sh");
    builder.arg("-lc");
    builder.arg(command);
    builder.cwd(cwd);
    let mut child = pair.slave.spawn_command(builder)?;
    let _ = killer_tx.send(child.clone_killer());
    drop(pair.slave);
    let mut reader = pair.master.try_clone_reader()?;
    let mut output = Vec::new();
    let mut buffer = [0u8; 8192];
    loop {
        let read = reader.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        output.extend_from_slice(&buffer[..read]);
        if output.len() > max_output_bytes.max(1024) {
            let excess = output.len() - max_output_bytes.max(1024);
            output.drain(..excess);
        }
    }
    let status = child.wait()?;
    if !status.success() {
        return Err(anyhow!("PTY command exited unsuccessfully: {status:?}"));
    }
    Ok(String::from_utf8_lossy(&output).into_owned())
}

#[cfg(unix)]
pub async fn run_command_with_cancel(
    command: &str,
    cwd: &std::path::Path,
    timeout: Duration,
    cancel: CancellationToken,
) -> Result<String> {
    run_command_with_cancel_bounded(command, cwd, timeout, cancel, 128 * 1024).await
}

#[cfg(unix)]
pub async fn run_command_with_cancel_bounded(
    command: &str,
    cwd: &std::path::Path,
    timeout: Duration,
    cancel: CancellationToken,
    max_output_bytes: usize,
) -> Result<String> {
    use tokio::process::Command;

    let mut process = Command::new("sh");
    process
        .arg("-lc")
        .arg(command)
        .current_dir(cwd)
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped());
    // SAFETY: setpgid only changes the child process before exec and does not
    // access Rust-managed memory.
    unsafe {
        process.pre_exec(|| {
            if libc::setpgid(0, 0) == -1 {
                return Err(std::io::Error::last_os_error());
            }
            Ok(())
        });
    }
    let mut child = process.spawn()?;
    let pid = child
        .id()
        .ok_or_else(|| anyhow!("shell process has no pid"))? as i32;
    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| anyhow!("missing stdout pipe"))?;
    let stderr = child
        .stderr
        .take()
        .ok_or_else(|| anyhow!("missing stderr pipe"))?;
    let max_tail = max_output_bytes.max(1024);
    let stdout_task = tokio::spawn(read_tail(stdout, max_tail));
    let stderr_task = tokio::spawn(read_tail(stderr, max_tail));

    let mut timed_out = false;
    let status = tokio::select! {
        result = child.wait() => result?,
        _ = tokio::time::sleep(timeout.max(Duration::from_millis(1))) => {
            timed_out = true;
            terminate_group(pid).await;
            child.wait().await?
        }
        _ = cancel.cancelled() => {
            terminate_group(pid).await;
            child.wait().await?
        }
    };
    let mut output = stdout_task.await??;
    output.extend(stderr_task.await??);
    let text = String::from_utf8_lossy(&output).into_owned();
    if cancel.is_cancelled() {
        return Err(anyhow!("command cancelled: {text}"));
    }
    if timed_out {
        return Err(anyhow!("command timed out: {text}"));
    }
    if !status.success() {
        return Err(anyhow!("command exited with {status}: {text}"));
    }
    Ok(text)
}

#[cfg(not(unix))]
pub async fn run_command_with_cancel(
    _command: &str,
    _cwd: &std::path::Path,
    _timeout: Duration,
    _cancel: CancellationToken,
) -> Result<String> {
    Err(anyhow!(
        "shell execution is supported only on Linux in runner V1"
    ))
}

#[cfg(not(unix))]
pub async fn run_command_with_cancel_bounded(
    _command: &str,
    _cwd: &std::path::Path,
    _timeout: Duration,
    _cancel: CancellationToken,
    _max_output_bytes: usize,
) -> Result<String> {
    Err(anyhow!(
        "shell execution is supported only on Linux in runner V1"
    ))
}

#[cfg(unix)]
async fn read_tail<R>(mut reader: R, max: usize) -> Result<Vec<u8>>
where
    R: tokio::io::AsyncRead + Unpin,
{
    use tokio::io::AsyncReadExt;
    let mut output = Vec::new();
    let mut buffer = [0u8; 8192];
    loop {
        let read = reader.read(&mut buffer).await?;
        if read == 0 {
            break;
        }
        output.extend_from_slice(&buffer[..read]);
        if output.len() > max {
            let excess = output.len() - max;
            output.drain(..excess);
        }
    }
    Ok(output)
}

#[cfg(unix)]
async fn terminate_group(pid: i32) {
    // Best effort escalation: descendants share the group created in pre_exec.
    unsafe {
        libc::killpg(pid, libc::SIGTERM);
    }
    tokio::time::sleep(Duration::from_millis(100)).await;
    unsafe {
        libc::killpg(pid, libc::SIGKILL);
    }
}
