//! Synthetic qualification probe; no production API or execution changes.
use engine::{scan, scan_with_callbacks, ScanError, ScanOptions};
use output::FileRecord;
use serde_json::json;
use std::cell::Cell;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};
use walker::{FilterOptions, WalkMode};

const PAYLOAD_BYTES: u64 = 256;
const PAYLOAD_SHA256: &str = "85e62acd750c4eb56b7b6a1d66dca5bfaac5f062608a1a893410d0288936c09a";

struct Class {
    name: String,
    extension: String,
    size: u64,
    sha256: String,
}
struct Fixture {
    mixed: bool,
    classes: Vec<Class>,
}
impl Fixture {
    fn load(root: &Path, mixed: bool) -> Result<Self, Box<dyn std::error::Error>> {
        if !mixed {
            return Ok(Self {
                mixed,
                classes: vec![Class {
                    name: "tiny".into(),
                    extension: "txt".into(),
                    size: PAYLOAD_BYTES,
                    sha256: PAYLOAD_SHA256.into(),
                }],
            });
        }
        let data = std::fs::read(root.join("fixture-spec.json"))?;
        if data.len() > 16384 {
            return Err("oversize fixture specification".into());
        }
        let value: serde_json::Value = serde_json::from_slice(&data)?;
        if value["schema"] != 1 || value["profile"] != "mixed" {
            return Err("fixture profile mismatch".into());
        }
        let values = value["classes"]
            .as_array()
            .ok_or("missing class specification")?;
        if values.len() != 8 {
            return Err("wrong fixture classes".into());
        }
        let names = [
            "empty", "medium", "large", "huge", "image", "document", "pdf", "tiny",
        ];
        let mut classes = Vec::new();
        for (i, v) in values.iter().enumerate() {
            let name = v["name"].as_str().ok_or("missing class name")?;
            let extension = v["extension"].as_str().ok_or("missing extension")?;
            let size = v["size"].as_u64().ok_or("missing class size")?;
            let sha = v["sha256"].as_str().ok_or("missing class digest")?;
            if name != names[i]
                || !["txt", "bin", "png", "docx", "pdf"].contains(&extension)
                || size > 16 << 20
                || sha.len() != 64
                || !sha.bytes().all(|b| b.is_ascii_hexdigit())
            {
                return Err("invalid fixture class".into());
            }
            classes.push(Class {
                name: name.into(),
                extension: extension.into(),
                size,
                sha256: sha.into(),
            });
        }
        // Fixed golden independently characterized from raw synthetic pixels;
        // the probe must not derive its expectation from this decoder at runtime.
        if phash::phash_u64(&root.join("alpha/shard-04/file-4.png"))? != 13089050091743192493 {
            return Err("PNG perceptual hash golden mismatch".into());
        }
        Ok(Self { mixed, classes })
    }

    fn index(&self, id: u64) -> usize {
        if self.mixed {
            (id % 128).min(7) as usize
        } else {
            0
        }
    }
}

struct Inventory {
    root: PathBuf,
    limit: u64,
    fixture: Arc<Fixture>,
    bytes: u64,
    classes: Vec<u64>,
    count: u64,
    sum: u128,
    xor: u64,
    first: Option<Duration>,
    started: Instant,
}
impl Inventory {
    fn new(root: &Path, limit: u64, fixture: Arc<Fixture>) -> Self {
        Self {
            root: root.to_owned(),
            limit,
            classes: vec![0; fixture.classes.len()],
            fixture,
            bytes: 0,
            count: 0,
            sum: 0,
            xor: 0,
            first: None,
            started: Instant::now(),
        }
    }
    fn accept(&mut self, record: FileRecord) -> io::Result<()> {
        let path = Path::new(&record.identity.path);
        let relative = path.strip_prefix(&self.root).map_err(io::Error::other)?;
        let file = path
            .file_stem()
            .and_then(|v| v.to_str())
            .ok_or_else(|| io::Error::other("missing filename"))?;
        let id: u64 = file
            .strip_prefix("file-")
            .ok_or_else(|| io::Error::other("unknown file"))?
            .parse()
            .map_err(io::Error::other)?;
        let class_index = self.fixture.index(id);
        let class = &self.fixture.classes[class_index];
        let expected = PathBuf::from(if id % 2 == 0 { "alpha" } else { "unicode-β" })
            .join(format!("shard-{:02}", id % 32))
            .join(format!("file-{id}.{}", class.extension));
        if id >= self.limit
            || relative != expected
            || record.identity.size != class.size
            || (class.name != "image" && record.error.is_some())
            || record.hashes.as_ref().and_then(|h| h.sha256.as_deref())
                != Some(class.sha256.as_str())
        {
            return Err(io::Error::other("fixture record mismatch"));
        }
        if self.fixture.mixed {
            if record
                .format
                .as_ref()
                .and_then(|f| f.mime.as_ref())
                .is_none()
            {
                return Err(io::Error::other("missing format extraction"));
            }
            let meta = record.meta.as_ref();
            let valid = match class.name.as_str() {
                "image" => {
                    record.error.is_none()
                        && meta.and_then(|m| m.phash) == Some(13089050091743192493)
                }
                "document" => meta.and_then(|m| m.docx.as_ref()).is_some_and(|d| {
                    d.creator.as_deref() == Some("Scanner Synthetic")
                        && d.last_modified_by.as_deref() == Some("Fixture generator")
                        && d.revision.as_deref() == Some("007")
                }),
                "pdf" => meta.and_then(|m| m.pdf.as_ref()).and_then(|p| p.page_count) == Some(32),
                _ => true,
            };
            if !valid {
                return Err(io::Error::other(format!(
                    "fixture extraction mismatch: {}",
                    class.name
                )));
            }
        }
        self.bytes += class.size;
        self.classes[class_index] += 1;
        self.first.get_or_insert_with(|| self.started.elapsed());
        self.count += 1;
        self.sum += u128::from(id);
        self.xor ^= id;
        Ok(())
    }
    fn complete(&self) -> bool {
        let xor = match (self.limit - 1) % 4 {
            0 => self.limit - 1,
            1 => 1,
            2 => self.limit,
            _ => 0,
        };
        let mut expected = vec![0; self.classes.len()];
        for id in 0..self.limit {
            expected[self.fixture.index(id)] += 1;
        }
        self.count == self.limit
            && self.classes == expected
            && self.sum == u128::from(self.limit) * u128::from(self.limit - 1) / 2
            && self.xor == xor
    }
}

struct Output<'a> {
    inventory: &'a mut Inventory,
    line: Vec<u8>,
    delay: Duration,
    fail_after: Option<u64>,
}
impl Write for Output<'_> {
    fn write(&mut self, bytes: &[u8]) -> io::Result<usize> {
        for byte in bytes {
            if *byte == b'\n' {
                if self.fail_after == Some(self.inventory.count) {
                    return Err(io::Error::new(
                        io::ErrorKind::BrokenPipe,
                        "synthetic output failure",
                    ));
                }
                let record = serde_json::from_slice(&self.line).map_err(io::Error::other)?;
                self.inventory.accept(record)?;
                self.line.clear();
                if !self.delay.is_zero() {
                    thread::sleep(self.delay);
                }
            } else {
                if self.line.len() >= 16384 {
                    return Err(io::Error::other("oversize record"));
                }
                self.line.push(*byte);
            }
        }
        Ok(bytes.len())
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if args.len() != 6 && args.len() != 7 {
        return Err(
            "usage: scale_probe FIXTURE MODE FILES DELAY_US STOP_AFTER standard|full".into(),
        );
    }
    let profile = args.get(6).map(String::as_str).unwrap_or("small");
    if !["small", "mixed"].contains(&profile) {
        return Err("unknown fixture profile".into());
    }
    let root = PathBuf::from(&args[0]).canonicalize()?;
    let mode = args[1].as_str();
    let files: u64 = args[2].parse()?;
    let delay = Duration::from_micros(args[3].parse()?);
    let stop_after: u64 = args[4].parse()?;
    if !(2..=1_000_000).contains(&files)
        || stop_after == 0
        || stop_after >= files
        || delay > Duration::from_millis(10)
        || ![
            "fast",
            "slow_callback",
            "slow_output",
            "cancel",
            "callback_error",
            "output_error",
        ]
        .contains(&mode)
    {
        return Err("invalid probe parameters".into());
    }
    if profile == "mixed" && files < 128 {
        return Err("mixed profile needs at least 128 files".into());
    }
    let fixture = Arc::new(Fixture::load(&root, profile == "mixed")?);
    let walk_mode = match args[5].as_str() {
        "standard" => WalkMode::Standard,
        "full" => WalkMode::Full,
        _ => return Err("invalid walker".into()),
    };
    let opts = ScanOptions {
        roots: vec![root.join("alpha"), root.join("unicode-β")],
        walk_mode,
        filter: FilterOptions {
            parents: false,
            git_ignore: false,
            ignore_files: false,
            skip_hidden: false,
            ..Default::default()
        },
        sniff_format: fixture.mixed,
        metadata_static: fixture.mixed,
        phash: fixture.mixed,
        sha256: true,
        ..Default::default()
    };
    let mut inventory = Inventory::new(&root, files, fixture.clone());
    let started = inventory.started;
    let requested = Cell::new(None::<Instant>);
    let caller = thread::current().id();
    let result: Result<(), ScanError> = if mode.ends_with("output") || mode == "output_error" {
        let mut writer = Output {
            inventory: &mut inventory,
            line: Vec::new(),
            delay: if mode == "slow_output" {
                delay
            } else {
                Duration::ZERO
            },
            fail_after: (mode == "output_error").then_some(stop_after),
        };
        let result = scan(&opts, &mut writer);
        if result.is_ok() && !writer.line.is_empty() {
            return Err("unfinished JSON output".into());
        }
        result
    } else {
        scan_with_callbacks(
            &opts,
            &mut || requested.get().is_some(),
            &mut |_| {
                assert_eq!(thread::current().id(), caller);
            },
            &mut |record| {
                assert_eq!(thread::current().id(), caller);
                if mode == "callback_error" && inventory.count == stop_after {
                    return Err(io::Error::other("synthetic callback failure").into());
                }
                inventory.accept(record)?;
                if mode == "cancel" && inventory.count == stop_after {
                    requested.set(Some(Instant::now()));
                }
                if mode == "slow_callback" {
                    thread::sleep(delay);
                }
                Ok(())
            },
        )
        .and_then(|counts| {
            if counts != (inventory.count, inventory.bytes) {
                return Err(io::Error::other("callback returned inconsistent counts").into());
            }
            Ok(())
        })
    };
    let finished = Instant::now();
    let expected_error = mode.ends_with("error");
    if expected_error {
        let expected = if mode == "output_error" {
            "synthetic output failure"
        } else {
            "synthetic callback failure"
        };
        match &result {
            Err(ScanError::Io(error)) if error.to_string() == expected => (),
            _ => return Err("missing expected error".into()),
        }
    } else {
        result?;
    }
    let partial = expected_error || mode == "cancel";
    if partial && inventory.count != stop_after || !partial && !inventory.complete() {
        return Err("inventory count/fingerprint mismatch".into());
    }
    let recovery_start = Instant::now();
    if partial {
        let mut recovered = Inventory::new(&root, files, fixture.clone());
        scan_with_callbacks(&opts, &mut || false, &mut |_| {}, &mut |r| {
            recovered.accept(r)?;
            Ok(())
        })?;
        if !recovered.complete() {
            return Err("post-stop recovery scan mismatch".into());
        }
    }
    println!(
        "{}",
        json!({"passed": true, "mode": mode, "walk_mode": args[5], "fixture_files": files,
        "files_delivered": inventory.count, "bytes_delivered": inventory.bytes,
        "profile": profile, "extraction_enabled": fixture.mixed,
        "qualified_png_records": if fixture.mixed { inventory.classes[4] } else { 0 },
        "image_capability": if fixture.mixed { "PNG decoded; fixed gradient perceptual hash checked" } else { "not exercised" },
        "delivered_by_class": fixture.classes.iter().zip(&inventory.classes).map(|(c, n)| (c.name.clone(), *n)).collect::<std::collections::BTreeMap<_, _>>(),
        "elapsed_ms": finished.duration_since(started).as_secs_f64() * 1000.0,
        "first_record_ms": inventory.first.map(|v| v.as_secs_f64() * 1000.0),
        "cancel_return_ms": requested.get().map(|v| finished.duration_since(v).as_secs_f64() * 1000.0),
        "expected_failure_observed": expected_error, "recovery_inventory_verified": partial,
        "recovery_ms": if partial { Some(recovery_start.elapsed().as_secs_f64() * 1000.0) } else { None },
        "fingerprint_scope": "count, byte totals, ID sum/XOR, root/path and SHA-256 checks; constant consumer state",
        "release_acceptance": false })
    );
    Ok(())
}
