use crate::types::VideoMeta;
use serde_json::Value;
use std::io::Read;
use std::path::Path;
use std::process::{Command, Stdio};
use std::thread;
use std::time::Duration;
use wait_timeout::ChildExt;

/// Wall-clock limit for `ffprobe` (corrupted files may hang indefinitely without this).
#[derive(Debug, Clone)]
pub struct FfprobeOptions {
    pub timeout: Duration,
}

impl Default for FfprobeOptions {
    fn default() -> Self {
        Self {
            timeout: Duration::from_secs(30),
        }
    }
}

/// Run `ffprobe` subprocess, parse JSON stdout, apply hard timeout.
pub fn read_video_ffprobe(path: &Path, opts: &FfprobeOptions) -> Option<VideoMeta> {
    let mut child = Command::new("ffprobe")
        .args([
            "-v",
            "quiet",
            "-print_format",
            "json",
            "-show_format",
            "-show_streams",
        ])
        .arg(path)
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .ok()?;

    let mut stdout = child.stdout.take()?;
    let reader = thread::spawn(move || {
        let mut buf = Vec::new();
        stdout.read_to_end(&mut buf).map(|_| buf)
    });

    let wait = child.wait_timeout(opts.timeout).ok()?;
    match wait {
        Some(status) if status.success() => {
            let bytes = match reader.join() {
                Ok(Ok(b)) => b,
                _ => return None,
            };
            let text = String::from_utf8_lossy(&bytes);
            let meta = parse_ffprobe_json(&text);
            if meta.codec_name.is_none()
                && meta.width.is_none()
                && meta.height.is_none()
                && meta.duration_secs.is_none()
            {
                None
            } else {
                Some(meta)
            }
        }
        Some(_) => {
            let _ = reader.join();
            None
        }
        None => {
            let _ = child.kill();
            let _ = reader.join();
            None
        }
    }
}

fn parse_ffprobe_json(json: &str) -> VideoMeta {
    let v: Value = match serde_json::from_str(json) {
        Ok(v) => v,
        Err(_) => return VideoMeta::default(),
    };

    let mut meta = VideoMeta::default();

    if let Some(fmt) = v.get("format").and_then(|f| f.as_object()) {
        if let Some(d) = fmt.get("duration").and_then(|x| x.as_str()) {
            meta.duration_secs = d.parse().ok();
        }
    }

    if let Some(streams) = v.get("streams").and_then(|s| s.as_array()) {
        for s in streams {
            let kind = s.get("codec_type").and_then(|x| x.as_str());
            if kind == Some("video") {
                meta.codec_name = s
                    .get("codec_name")
                    .and_then(|x| x.as_str())
                    .map(String::from);
                meta.width = s.get("width").and_then(|x| x.as_u64()).map(|u| u as u32);
                meta.height = s.get("height").and_then(|x| x.as_u64()).map(|u| u as u32);
                break;
            }
        }
    }

    meta
}
