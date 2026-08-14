use anyhow::{Context, Result, anyhow};
use globset::{Glob, GlobSetBuilder};
use regex::Regex;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    fs,
    io::Read,
    path::{Path, PathBuf},
};

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
pub struct FileSnapshot {
    pub path: String,
    pub tag: String,
    pub digest: String,
    pub lines: Vec<String>,
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
pub struct ToolResult {
    pub ok: bool,
    pub content: String,
    #[serde(default)]
    pub snapshot: Option<FileSnapshot>,
    #[serde(default)]
    pub warnings: Vec<String>,
}

#[derive(Clone, Debug)]
pub struct WorkspaceTools {
    root: PathBuf,
    max_output_bytes: usize,
    max_file_bytes: u64,
}

impl WorkspaceTools {
    pub fn new(root: impl AsRef<Path>, max_output_bytes: usize) -> Result<Self> {
        let root = fs::canonicalize(root.as_ref()).context("workspace root does not exist")?;
        if !root.is_dir() {
            return Err(anyhow!("workspace root is not a directory"));
        }
        Ok(Self {
            root,
            max_output_bytes: max_output_bytes.max(1024),
            // Keep native search/read operations bounded even when a manifest
            // requests a large output budget.
            max_file_bytes: 8 * 1024 * 1024,
        })
    }

    pub fn read(&self, path: &str) -> ToolResult {
        let resolved = match self.resolve(path) {
            Ok(path) => path,
            Err(error) => return failure(error),
        };
        let content = match read_bounded_text(&resolved, self.max_file_bytes) {
            Ok(content) => content,
            Err(error) => return failure(error.to_string()),
        };
        let snapshot = snapshot(path, &content);
        ToolResult {
            ok: true,
            content: truncate(content, self.max_output_bytes),
            snapshot: Some(snapshot),
            warnings: Vec::new(),
        }
    }

    pub fn grep(&self, pattern: &str, glob: Option<&str>) -> ToolResult {
        let regex = match Regex::new(pattern) {
            Ok(regex) => regex,
            Err(error) => return failure(error.to_string()),
        };
        let matcher = match glob {
            Some(glob) => match Glob::new(glob).and_then(|glob| {
                let mut builder = GlobSetBuilder::new();
                builder.add(glob);
                builder.build()
            }) {
                Ok(matcher) => Some(matcher),
                Err(error) => return failure(error.to_string()),
            },
            None => None,
        };
        let mut matches = Vec::new();
        let mut stack = vec![self.root.clone()];
        while let Some(dir) = stack.pop() {
            let entries = match fs::read_dir(&dir) {
                Ok(entries) => entries,
                Err(_) => continue,
            };
            for entry in entries.flatten() {
                let path = entry.path();
                let Ok(metadata) = fs::symlink_metadata(&path) else {
                    continue;
                };
                if metadata.file_type().is_symlink() {
                    continue;
                }
                let relative = path
                    .strip_prefix(&self.root)
                    .unwrap_or(&path)
                    .to_string_lossy()
                    .replace('\\', "/");
                if metadata.is_dir() {
                    if !relative.starts_with(".git") && !relative.starts_with("target") {
                        stack.push(path);
                    }
                    continue;
                }
                if matcher
                    .as_ref()
                    .is_some_and(|matcher| !matcher.is_match(&relative))
                {
                    continue;
                }
                if metadata.len() > self.max_file_bytes {
                    continue;
                }
                let Ok(content) = read_bounded_text(&path, self.max_file_bytes) else {
                    continue;
                };
                for (line_number, line) in content.lines().enumerate() {
                    if regex.is_match(line) {
                        matches.push(format!("{relative}:{}:{line}", line_number + 1));
                    }
                    if matches.join("\n").len() >= self.max_output_bytes {
                        return success(truncate(matches.join("\n"), self.max_output_bytes));
                    }
                }
            }
        }
        success(matches.join("\n"))
    }

    pub fn glob(&self, pattern: &str) -> ToolResult {
        let matcher = match Glob::new(pattern) {
            Ok(glob) => match {
                let mut builder = GlobSetBuilder::new();
                builder.add(glob);
                builder.build()
            } {
                Ok(matcher) => matcher,
                Err(error) => return failure(error.to_string()),
            },
            Err(error) => return failure(error.to_string()),
        };
        let mut result = Vec::new();
        let mut stack = vec![self.root.clone()];
        while let Some(dir) = stack.pop() {
            for entry in fs::read_dir(dir).into_iter().flatten().flatten() {
                let path = entry.path();
                let Ok(metadata) = fs::symlink_metadata(&path) else {
                    continue;
                };
                if metadata.file_type().is_symlink() {
                    continue;
                }
                let relative = path
                    .strip_prefix(&self.root)
                    .unwrap_or(&path)
                    .to_string_lossy()
                    .replace('\\', "/");
                if metadata.is_dir() {
                    if !relative.starts_with(".git") && !relative.starts_with("target") {
                        stack.push(path);
                    }
                } else if matcher.is_match(&relative) {
                    result.push(relative);
                }
            }
        }
        result.sort();
        success(truncate(result.join("\n"), self.max_output_bytes))
    }

    pub fn write(&self, path: &str, content: &str) -> ToolResult {
        let resolved = match self.resolve(path) {
            Ok(path) => path,
            Err(error) => return failure(error),
        };
        if let Some(parent) = resolved.parent() {
            if let Err(error) = fs::create_dir_all(parent) {
                return failure(error.to_string());
            }
        }
        if let Err(error) = atomic_write(&resolved, content.as_bytes()) {
            return failure(error.to_string());
        }
        ToolResult {
            ok: true,
            content: format!("wrote {path}"),
            snapshot: Some(snapshot(path, content)),
            warnings: Vec::new(),
        }
    }

    pub fn hashline_edit(
        &self,
        path: &str,
        expected_tag: &str,
        replacements: &[(usize, String)],
    ) -> ToolResult {
        let resolved = match self.resolve(path) {
            Ok(path) => path,
            Err(error) => return failure(error),
        };
        let original = match fs::read_to_string(&resolved) {
            Ok(content) => content,
            Err(error) => return failure(error.to_string()),
        };
        let current = snapshot(path, &original);
        if current.tag != expected_tag {
            return failure(format!(
                "stale snapshot for {path}: expected {expected_tag}, current {}",
                current.tag
            ));
        }
        let mut seen = std::collections::HashSet::new();
        if replacements.iter().any(|(line, _)| !seen.insert(*line)) {
            return failure("overlapping hashline replacements are not allowed");
        }
        let mut lines = original
            .lines()
            .map(ToString::to_string)
            .collect::<Vec<_>>();
        for (line, replacement) in replacements.iter().rev() {
            if *line == 0 || *line > lines.len() {
                return failure(format!("line {line} is outside {path}"));
            }
            lines[*line - 1] = replacement.clone();
        }
        let updated = lines.join("\n") + if original.ends_with('\n') { "\n" } else { "" };
        if let Err(error) = atomic_write(&resolved, updated.as_bytes()) {
            return failure(error.to_string());
        }
        ToolResult {
            ok: true,
            content: format!("edited {path}"),
            snapshot: Some(snapshot(path, &updated)),
            warnings: Vec::new(),
        }
    }

    fn resolve(&self, path: &str) -> Result<PathBuf, String> {
        let candidate = self.root.join(path);
        let mut current = self.root.clone();
        for component in candidate
            .strip_prefix(&self.root)
            .map_err(|_| "path escapes workspace root".to_string())?
            .components()
        {
            current.push(component.as_os_str());
            if let Ok(metadata) = fs::symlink_metadata(&current)
                && metadata.file_type().is_symlink()
            {
                return Err("symbolic links are not allowed".into());
            }
        }
        let normalized = if candidate.exists() {
            fs::canonicalize(&candidate).map_err(|error| error.to_string())?
        } else {
            let parent = candidate
                .parent()
                .ok_or_else(|| "path has no parent".to_string())?;
            let canonical_parent = fs::canonicalize(parent).map_err(|error| error.to_string())?;
            lexical_normalize(
                &canonical_parent.join(
                    candidate
                        .file_name()
                        .ok_or_else(|| "path has no file name".to_string())?,
                ),
            )
        };
        if !normalized.starts_with(&self.root) {
            return Err("path escapes workspace root".into());
        }
        Ok(normalized)
    }
}

fn snapshot(path: &str, content: &str) -> FileSnapshot {
    let mut digest = Sha256::new();
    digest.update(content.as_bytes());
    let digest = hex::encode(digest.finalize());
    FileSnapshot {
        path: path.into(),
        tag: digest[..8].to_uppercase(),
        digest,
        lines: content.lines().map(|line| line_digest(line)).collect(),
    }
}

fn line_digest(line: &str) -> String {
    let mut digest = Sha256::new();
    digest.update(line.as_bytes());
    hex::encode(&digest.finalize()[..4])
}
fn atomic_write(path: &Path, bytes: &[u8]) -> Result<()> {
    let temp = path.with_extension(format!("gantry-{}", uuid::Uuid::new_v4()));
    fs::write(&temp, bytes)?;
    fs::rename(&temp, path)?;
    Ok(())
}

fn read_bounded_text(path: &Path, max_bytes: u64) -> Result<String> {
    let file = fs::File::open(path)?;
    let mut bytes = Vec::new();
    file.take(max_bytes.saturating_add(1))
        .read_to_end(&mut bytes)?;
    if bytes.len() as u64 > max_bytes {
        return Err(anyhow!("file exceeds native tool size limit"));
    }
    String::from_utf8(bytes).context("file is not valid UTF-8")
}
fn lexical_normalize(path: &Path) -> PathBuf {
    let mut output = PathBuf::new();
    for component in path.components() {
        match component {
            std::path::Component::ParentDir => {
                output.pop();
            }
            std::path::Component::CurDir => {}
            component => output.push(component.as_os_str()),
        }
    }
    output
}
fn truncate(value: String, max: usize) -> String {
    if value.len() <= max {
        value
    } else {
        format!(
            "{}\n[truncated; full output unavailable]",
            &value[value.len() - max..]
        )
    }
}
fn success(content: String) -> ToolResult {
    ToolResult {
        ok: true,
        content,
        snapshot: None,
        warnings: Vec::new(),
    }
}
fn failure(error: impl Into<String>) -> ToolResult {
    ToolResult {
        ok: false,
        content: error.into(),
        snapshot: None,
        warnings: Vec::new(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn workspace() -> (WorkspaceTools, PathBuf) {
        let root = std::env::temp_dir().join(format!(
            "gantry-tools-{}",
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir_all(&root).unwrap();
        (WorkspaceTools::new(&root, 4096).unwrap(), root)
    }

    #[test]
    fn hashline_rejects_stale_snapshot() {
        let (tools, root) = workspace();
        tools.write("a.txt", "one\ntwo\n");
        let read = tools.read("a.txt");
        fs::write(root.join("a.txt"), "changed\ntwo\n").unwrap();
        let result =
            tools.hashline_edit("a.txt", &read.snapshot.unwrap().tag, &[(1, "new".into())]);
        assert!(!result.ok);
    }

    #[test]
    fn paths_cannot_escape_workspace() {
        let (tools, _root) = workspace();
        assert!(!tools.read("../outside").ok);
    }

    #[cfg(unix)]
    #[test]
    fn search_does_not_follow_symbolic_links() {
        let (tools, root) = workspace();
        let outside = root
            .parent()
            .unwrap()
            .join(format!("gantry-tools-outside-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&outside).unwrap();
        fs::write(outside.join("secret.txt"), "secret").unwrap();
        std::os::unix::fs::symlink(&outside, root.join("linked")).unwrap();
        assert!(!tools.glob("**/*").content.contains("secret.txt"));
        assert!(!tools.grep("secret", None).content.contains("secret"));
        let _ = fs::remove_file(root.join("linked"));
        let _ = fs::remove_dir_all(outside);
    }

    #[test]
    fn read_rejects_files_above_native_limit() {
        let (tools, root) = workspace();
        let path = root.join("large.txt");
        let file = fs::File::create(&path).unwrap();
        file.set_len(8 * 1024 * 1024 + 1).unwrap();
        assert!(!tools.read("large.txt").ok);
    }
}
