use regex::Regex;
use thiserror::Error;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum PolicyError {
    #[error("shell execution is disabled")]
    ShellDisabled,
    #[error("command denied by local policy: {0}")]
    Denied(String),
    #[error("command must use the dedicated tool: {0}")]
    Intercepted(String),
}

pub fn check_command(
    command: &str,
    allow_shell: bool,
    denied: &[String],
    interceptors: &[String],
) -> Result<(), PolicyError> {
    if !allow_shell {
        return Err(PolicyError::ShellDisabled);
    }
    for pattern in denied {
        if Regex::new(pattern)
            .ok()
            .is_some_and(|regex| regex.is_match(command))
        {
            return Err(PolicyError::Denied(pattern.clone()));
        }
    }
    for pattern in interceptors {
        if Regex::new(pattern)
            .ok()
            .is_some_and(|regex| regex.is_match(command))
        {
            return Err(PolicyError::Intercepted(pattern.clone()));
        }
    }
    for fragment in command_fragments(command) {
        let trimmed = fragment.trim_start();
        if trimmed.starts_with("cat ")
            || trimmed == "cat"
            || trimmed.starts_with("rg ")
            || trimmed == "rg"
            || trimmed.starts_with("find ")
            || trimmed == "find"
            || trimmed.starts_with("sed -i")
            || contains_unquoted_redirection(trimmed)
        {
            return Err(PolicyError::Intercepted(trimmed.to_string()));
        }
    }
    Ok(())
}

fn command_fragments(command: &str) -> Vec<String> {
    let mut fragments = Vec::new();
    let mut current = String::new();
    let mut quote = None;
    for character in command.chars() {
        match (quote, character) {
            (None, '\'' | '"') => {
                quote = Some(character);
                current.push(character);
            }
            (Some(open), character) if character == open => {
                quote = None;
                current.push(character);
            }
            (None, ';' | '|' | '&' | '\n') => {
                if !current.trim().is_empty() {
                    fragments.push(std::mem::take(&mut current));
                }
            }
            _ => current.push(character),
        }
    }
    if !current.trim().is_empty() {
        fragments.push(current);
    }
    fragments
}

fn contains_unquoted_redirection(command: &str) -> bool {
    let mut quote = None;
    for character in command.chars() {
        match (quote, character) {
            (None, '\'' | '"') => quote = Some(character),
            (Some(open), character) if character == open => quote = None,
            (None, '>' | '<') => return true,
            _ => {}
        }
    }
    false
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn dangerous_commands_fail_closed() {
        assert_eq!(
            check_command("rm -rf /", true, &["rm\\s+-rf".into()], &[]),
            Err(PolicyError::Denied("rm\\s+-rf".into()))
        );
        assert!(matches!(
            check_command("rg foo", true, &[], &["^rg\\s".into()]),
            Err(PolicyError::Intercepted(_))
        ));
        assert!(matches!(
            check_command("echo ok; cat file.txt", true, &[], &[]),
            Err(PolicyError::Intercepted(_))
        ));
        assert!(matches!(
            check_command("printf '>'", true, &[], &[]),
            Ok(())
        ));
    }
}
