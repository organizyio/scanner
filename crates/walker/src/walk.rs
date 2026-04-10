use crate::filter::{apply_walk_builder, FilterOptions};
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use thiserror::Error;
use walkdir::WalkDir;

#[derive(Debug, Clone, Copy, Default)]
pub enum WalkMode {
    #[default]
    Standard,
    Full,
}

#[derive(Debug, Error)]
pub enum WalkError {
    #[error(transparent)]
    Ignore(#[from] ignore::Error),
    #[error(transparent)]
    Walkdir(#[from] walkdir::Error),
}

pub struct WalkOutcome {
    pub files_seen: usize,
}

/// Walk each root in order, invoking `on_file` for every file in parallel (rayon workers per `ignore`).
pub fn walk_roots_fn(
    roots: &[PathBuf],
    opts: &FilterOptions,
    mode: WalkMode,
    on_file: impl Fn(PathBuf) + Sync + Send + Clone + 'static,
) -> Result<WalkOutcome, WalkError> {
    let files_seen = Arc::new(AtomicUsize::new(0));

    match mode {
        WalkMode::Standard => {
            for root in roots {
                let wb = apply_walk_builder(root, opts)?;
                let walker = wb.build_parallel();
                let seen = Arc::clone(&files_seen);

                walker.run({
                    let on_file = on_file.clone();
                    move || {
                        let on_file = on_file.clone();
                        let seen = Arc::clone(&seen);
                        Box::new(move |res| {
                            if let Ok(entry) = res {
                                if entry.file_type().map(|ft| ft.is_file()).unwrap_or(false) {
                                    seen.fetch_add(1, Ordering::Relaxed);
                                    on_file(entry.path().to_path_buf());
                                }
                            }
                            ignore::WalkState::Continue
                        })
                    }
                });
            }
        }
        WalkMode::Full => {
            for root in roots {
                for entry in WalkDir::new(root)
                    .follow_links(false)
                    .into_iter()
                    .filter_map(Result::ok)
                {
                    if entry.file_type().is_file() {
                        files_seen.fetch_add(1, Ordering::Relaxed);
                        on_file(entry.path().to_path_buf());
                    }
                }
            }
        }
    }

    Ok(WalkOutcome {
        files_seen: files_seen.load(Ordering::SeqCst),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;
    use std::fs;
    use std::path::PathBuf;
    use std::sync::{Arc, Mutex};
    use tempfile::tempdir;

    #[test]
    fn standard_skips_gitignored_and_full_includes() {
        let dir = tempdir().expect("temp dir");
        let root = dir.path();
        fs::write(root.join(".ignore"), "node_modules/\n").expect("write ignore");
        fs::create_dir_all(root.join("node_modules")).expect("create node_modules");
        fs::write(root.join("node_modules").join("a.js"), "x").expect("write ignored file");
        fs::write(root.join("keep.txt"), "k").expect("write keep file");

        let roots = vec![root.to_path_buf()];
        let opts = FilterOptions::default();
        let standard_paths: Arc<Mutex<HashSet<PathBuf>>> = Arc::new(Mutex::new(HashSet::new()));
        let standard_paths_ref = Arc::clone(&standard_paths);
        walk_roots_fn(&roots, &opts, WalkMode::Standard, move |p| {
            standard_paths_ref.lock().expect("lock").insert(p);
        })
        .expect("standard walk");

        let full_paths: Arc<Mutex<HashSet<PathBuf>>> = Arc::new(Mutex::new(HashSet::new()));
        let full_paths_ref = Arc::clone(&full_paths);
        walk_roots_fn(&roots, &opts, WalkMode::Full, move |p| {
            full_paths_ref.lock().expect("lock").insert(p);
        })
        .expect("full walk");

        let standard_paths = standard_paths.lock().expect("lock");
        let full_paths = full_paths.lock().expect("lock");
        assert!(standard_paths.contains(&root.join("keep.txt")));
        assert!(!standard_paths.contains(&root.join("node_modules").join("a.js")));
        assert!(full_paths.contains(&root.join("keep.txt")));
        assert!(full_paths.contains(&root.join("node_modules").join("a.js")));
    }
}
