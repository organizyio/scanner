use crate::filter::{apply_walk_builder, FilterOptions};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
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
    walk_roots_cancellable_fn(roots, opts, mode, Arc::new(AtomicBool::new(false)), on_file)
}

/// Walk with cooperative cancellation. Active filesystem calls are not preempted.
pub fn walk_roots_cancellable_fn(
    roots: &[PathBuf],
    opts: &FilterOptions,
    mode: WalkMode,
    cancelled: Arc<AtomicBool>,
    on_file: impl Fn(PathBuf) + Sync + Send + Clone + 'static,
) -> Result<WalkOutcome, WalkError> {
    let files_seen = Arc::new(AtomicUsize::new(0));

    match mode {
        WalkMode::Standard => {
            for root in roots {
                if cancelled.load(Ordering::Relaxed) {
                    break;
                }
                let wb = apply_walk_builder(root, opts)?;
                let walker = wb.build_parallel();
                let seen = Arc::clone(&files_seen);

                walker.run({
                    let on_file = on_file.clone();
                    let cancelled = Arc::clone(&cancelled);
                    move || {
                        let on_file = on_file.clone();
                        let seen = Arc::clone(&seen);
                        let cancelled = Arc::clone(&cancelled);
                        Box::new(move |res| {
                            if cancelled.load(Ordering::Relaxed) {
                                return ignore::WalkState::Quit;
                            }
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
                if cancelled.load(Ordering::Relaxed) {
                    break;
                }
                let mut entries = WalkDir::new(root).follow_links(false).into_iter();
                while !cancelled.load(Ordering::Relaxed) {
                    let Some(entry) = entries.next() else { break };
                    let Ok(entry) = entry else { continue };
                    if cancelled.load(Ordering::Relaxed) {
                        break;
                    }
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
    fn pre_cancelled_walk_does_no_work_in_either_mode() {
        for mode in [WalkMode::Standard, WalkMode::Full] {
            let result = walk_roots_cancellable_fn(
                &[PathBuf::from("missing")],
                &FilterOptions::default(),
                mode,
                Arc::new(AtomicBool::new(true)),
                |_| panic!("cancelled walk delivered a file"),
            )
            .unwrap();
            assert_eq!(result.files_seen, 0);
        }
    }

    #[test]
    fn standard_walk_observes_callback_cancellation() {
        let dir = tempdir().unwrap();
        for i in 0..1000 {
            fs::write(dir.path().join(i.to_string()), "x").unwrap();
        }
        let cancelled = Arc::new(AtomicBool::new(false));
        let flag = cancelled.clone();
        let result = walk_roots_cancellable_fn(
            &[dir.path().to_owned(), dir.path().to_owned()],
            &FilterOptions::default(),
            WalkMode::Standard,
            cancelled,
            move |_| {
                flag.store(true, Ordering::Relaxed);
            },
        )
        .unwrap();
        assert!(result.files_seen > 0);
        assert!(result.files_seen < 1000);
    }

    #[test]
    fn full_walk_stops_between_files_and_roots() {
        let dir = tempdir().unwrap();
        for i in 0..20 {
            fs::write(dir.path().join(i.to_string()), "x").unwrap();
        }
        let cancelled = Arc::new(AtomicBool::new(false));
        let flag = cancelled.clone();
        let result = walk_roots_cancellable_fn(
            &[dir.path().to_owned(), dir.path().to_owned()],
            &FilterOptions::default(),
            WalkMode::Full,
            cancelled,
            move |_| {
                flag.store(true, Ordering::Relaxed);
            },
        )
        .unwrap();
        assert_eq!(result.files_seen, 1);
    }

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
