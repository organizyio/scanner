use ignore::overrides::OverrideBuilder;
use ignore::WalkBuilder;
use std::path::Path;

/// Options for `WalkBuilder` (gitignore, hidden files, custom globs).
#[derive(Debug, Clone)]
pub struct FilterOptions {
    /// When true, respect `.gitignore` / `.git/info/exclude` (default: true).
    pub git_ignore: bool,
    /// When true, respect `.ignore` files (default: true).
    pub ignore_files: bool,
    /// When true, skip hidden files and directories (default: true).
    pub skip_hidden: bool,
    /// When true, read ignore rules from parent directories (default: true).
    pub parents: bool,
    /// Override globs in `gitignore` syntax (e.g. `!*.tmp` or `target/`).
    pub overrides: Vec<String>,
}

impl Default for FilterOptions {
    fn default() -> Self {
        Self {
            git_ignore: true,
            ignore_files: true,
            skip_hidden: true,
            parents: true,
            overrides: Vec::new(),
        }
    }
}

/// Stable hash for [`crate::Checkpoint::config_hash`].
pub fn config_hash(opts: &FilterOptions) -> u64 {
    use std::hash::{Hash, Hasher};
    let mut h = std::collections::hash_map::DefaultHasher::new();
    opts.git_ignore.hash(&mut h);
    opts.ignore_files.hash(&mut h);
    opts.skip_hidden.hash(&mut h);
    opts.parents.hash(&mut h);
    for line in &opts.overrides {
        line.hash(&mut h);
    }
    h.finish()
}

/// Configure a `WalkBuilder` for `root` using `opts`.
pub fn apply_walk_builder(root: &Path, opts: &FilterOptions) -> Result<WalkBuilder, ignore::Error> {
    let mut wb = WalkBuilder::new(root);
    wb.git_ignore(opts.git_ignore)
        .ignore(opts.ignore_files)
        .hidden(opts.skip_hidden)
        .parents(opts.parents);

    if !opts.overrides.is_empty() {
        let mut ob = OverrideBuilder::new(root);
        for pat in &opts.overrides {
            ob.add(pat)?;
        }
        wb.overrides(ob.build()?);
    }

    Ok(wb)
}
